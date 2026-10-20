//go:build windows

package runner

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestClassifyWindowsVersion(t *testing.T) {
	tests := []struct {
		name        string
		major       uint32
		build       uint32
		productType byte
		expected    HostEnvironment
	}{
		{name: "Windows 10 before ConPTY", major: 10, build: 17134, productType: 1},
		{name: "Windows 10 1809", major: 10, build: 17763, productType: 1, expected: HostEnvironmentWindows10},
		{name: "Windows 10 22H2", major: 10, build: 19045, productType: 1, expected: HostEnvironmentWindows10},
		{name: "Windows Server", major: 10, build: 17763, productType: 3},
		{name: "Windows 10 last build before Windows 11", major: 10, build: 21999, productType: 1, expected: HostEnvironmentWindows10},
		{name: "Windows 11", major: 10, build: 22000, productType: 1, expected: HostEnvironmentWindows11},
		{name: "Windows 11 current host build", major: 10, build: 26200, productType: 1, expected: HostEnvironmentWindows11},
		{name: "Windows Server with Windows 11 build range", major: 10, build: 26100, productType: 3},
		{name: "Windows domain controller", major: 10, build: 26100, productType: 2},
		{name: "future major version", major: 11, build: 100, productType: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expected, classifyWindowsVersion(test.major, test.build, test.productType))
		})
	}
}

func TestShouldUseConPTYRequiresAllowlistAndAPI(t *testing.T) {
	enabled := Config{ConPTYEnvironments: []HostEnvironment{HostEnvironmentWindows10}}

	assert.True(t, shouldUseConPTY(enabled, HostEnvironmentWindows10, true))
	assert.False(t, shouldUseConPTY(Config{}, HostEnvironmentWindows10, true))
	assert.False(t, shouldUseConPTY(enabled, "", true))
	assert.False(t, shouldUseConPTY(enabled, HostEnvironmentWindows10, false))
	assert.False(t, shouldUseConPTY(enabled, HostEnvironmentWindows11, true))

	enabled.ConPTYEnvironments = append(enabled.ConPTYEnvironments, HostEnvironmentWindows11)
	assert.True(t, shouldUseConPTY(enabled, HostEnvironmentWindows11, true))
	assert.False(t, shouldUseConPTY(enabled, HostEnvironmentWindows11, false))
}

func TestConPTYOutputFiltersHostInitializationLines(t *testing.T) {
	input := bytes.NewBufferString(
		"\x1b[2J\x1b[H\x1b]0;C:\\Windows\\System32\\cmd.exe\a\x1b[?25h\r\ncommand output\r\n",
	)
	sut := &conPTYOutput{reader: bufio.NewReader(input)}

	output, err := io.ReadAll(sut)

	require.NoError(t, err)
	assert.Equal(t, "command output\r\n", string(output))
}

func TestConPTYOutputFiltersWindows11ViewportInitialization(t *testing.T) {
	// Captured from Windows 11 build 26200 with the configured 1000-row console.
	header := "\x1b[?9001h\x1b[?1004h\x1b[?25l\x1b[2J\x1b[m\x1b[H\r\n"
	title := "\x1b]0;C:\\Windows\\System32\\cmd.exe\a\x1b[?25h"
	for _, inlineOutput := range []bool{false, true} {
		t.Run(fmt.Sprintf("inline=%t", inlineOutput), func(t *testing.T) {
			firstLine := title + "\r\n"
			expected := "\r\ncommand output\r\n\r\n"
			if inlineOutput {
				firstLine = title + "inline output"
				expected = firstLine + expected
			}
			input := header + strings.Repeat("\r\n", 998) + "\x1b[H" + firstLine + "\r\ncommand output\r\n\r\n"
			sut := &conPTYOutput{reader: bufio.NewReader(strings.NewReader(input))}

			output, err := io.ReadAll(sut)

			require.NoError(t, err)
			assert.Equal(t, expected, string(output), "preserve command blank lines after host initialization")
			assert.NotContains(t, string(output), string(conPTYCursorHome))
		})
	}
}

func TestWindowsProcessUsesTerminalOnlyWhenConfigured(t *testing.T) {
	host := currentWindowsHostEnvironment()
	if host == "" || !isConPTYAvailable() {
		t.Skip("the current host is not a supported Windows ConPTY environment")
	}

	terminalProcess := newCommandProcess(
		`powershell -NoProfile -Command Write-Output ([Console]::IsOutputRedirected)`,
		`C:\`,
		os.Environ(),
		Config{ConPTYEnvironments: []HostEnvironment{host}},
	)
	terminalOutput, err := runAndReadProcess(terminalProcess)
	require.NoError(t, err)
	assert.Contains(t, terminalOutput, "False")
	assert.NotContains(t, terminalOutput, string(conPTYClearScreen))
	assert.NotContains(t, terminalOutput, string(conPTYCursorHome))
	assert.Less(t, strings.Count(terminalOutput, "\n"), 10, "host initialization must not flood the terminal")

	pipeProcess := newCommandProcess(
		`powershell -NoProfile -Command Write-Output ([Console]::IsOutputRedirected)`,
		`C:\`,
		os.Environ(),
		Config{},
	)
	pipeOutput, err := runAndReadProcess(pipeProcess)
	require.NoError(t, err)
	assert.Contains(t, pipeOutput, "True")
}

func TestWindowsConPTYProcessReturnsCommandFailure(t *testing.T) {
	host := currentWindowsHostEnvironment()
	if host == "" || !isConPTYAvailable() {
		t.Skip("the current host is not a supported Windows ConPTY environment")
	}

	process := newCommandProcess(
		"definitely-not-a-real-command-12345",
		`C:\`,
		os.Environ(),
		Config{ConPTYEnvironments: []HostEnvironment{host}},
	)
	_, err := runAndReadProcess(process)
	assert.Error(t, err)
}

func TestCommandInterpreterDefaultsToCmd(t *testing.T) {
	t.Setenv("COMSPEC", "")
	assert.Equal(t, "cmd.exe", commandInterpreter())
}

func TestTaskkillPreservesFailureDetails(t *testing.T) {
	// An invalid PID cannot target a real process and produces a native diagnostic
	// regardless of the host's display language.
	err := runTaskkill("/PID", "not-a-process-id")

	require.Error(t, err)
	var exitErr *exec.ExitError
	require.ErrorAs(t, err, &exitErr)
	assert.Contains(t, err.Error(), "not-a-process-id")
	parts := strings.SplitN(err.Error(), exitErr.Error()+":", 2)
	require.Len(t, parts, 2)
	assert.NotEmpty(t, strings.TrimSpace(parts[1]))
}

func TestWindowsProcessStopReleasesDescendantPort(t *testing.T) {
	executable, err := os.Executable()
	require.NoError(t, err)
	host := currentWindowsHostEnvironment()

	for _, terminal := range []bool{false, true} {
		t.Run(fmt.Sprintf("ConPTY=%t", terminal), func(t *testing.T) {
			config := Config{}
			if terminal {
				if host == "" || !isConPTYAvailable() {
					t.Skip("the current host is not a supported Windows ConPTY environment")
				}
				config.ConPTYEnvironments = []HostEnvironment{host}
			}

			environment := append(os.Environ(), "GOMANDER_PROCESS_TREE_HELPER=parent")
			process := newCommandProcess(
				`.\`+filepath.Base(executable)+" -test.run=TestWindowsProcessTreeHelper",
				filepath.Dir(executable), environment, config,
			)
			readers, err := process.Start()
			require.NoError(t, err)

			ready := make(chan []string, 1)
			marker := regexp.MustCompile(`GOMANDER_LISTENER:(\d+):(127\.0\.0\.1:\d+)`)
			var outputWorkers sync.WaitGroup
			for _, reader := range readers {
				outputWorkers.Add(1)
				go func(reader io.ReadCloser) {
					defer outputWorkers.Done()
					scanner := bufio.NewScanner(reader)
					for scanner.Scan() {
						if strings.TrimSpace(scanner.Text()) != "" {
							t.Log(scanner.Text())
						}
						if match := marker.FindStringSubmatch(scanner.Text()); match != nil {
							ready <- match
						}
					}
				}(reader)
			}
			exited := make(chan struct{})
			go func() {
				_ = process.Wait()
				close(exited)
			}()
			t.Cleanup(func() {
				if !alreadyExited(exited) {
					_ = runTaskkill("/F", "/T", "/PID", strconv.Itoa(process.PID()))
				}
				closeReaders(readers...)
				outputWorkers.Wait()
				select {
				case <-exited:
				case <-time.After(10 * time.Second):
					t.Error("the command did not exit during cleanup")
				}
			})

			var listener []string
			select {
			case listener = <-ready:
			case <-time.After(15 * time.Second):
				t.Fatal("the descendant did not start its HTTP listener")
			}
			childPID, err := strconv.Atoi(listener[1])
			require.NoError(t, err)
			// Keep a handle to this exact child so PID reuse cannot affect cleanup
			// or the assertion that the descendant really exited.
			child, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(childPID))
			require.NoError(t, err)
			t.Cleanup(func() {
				_ = windows.TerminateProcess(child, 1)
				_ = windows.CloseHandle(child)
			})
			client := &http.Client{Timeout: 3 * time.Second}
			response, err := client.Get("http://" + listener[2])
			require.NoError(t, err)
			body, err := io.ReadAll(response.Body)
			require.NoError(t, response.Body.Close())
			require.NoError(t, err)
			require.Equal(t, "ready", string(body))
			client.CloseIdleConnections()

			require.NoError(t, stopProcessGracefully(process, exited))
			select {
			case <-exited:
			case <-time.After(10 * time.Second):
				t.Fatal("the command did not exit after stopping")
			}
			waitResult, err := windows.WaitForSingleObject(child, 5000)
			require.NoError(t, err)
			require.EqualValues(t, windows.WAIT_OBJECT_0, waitResult, "the descendant must exit too")

			rebound, err := net.Listen("tcp", listener[2])
			require.NoError(t, err, "stopping must release the descendant's listening port")
			require.NoError(t, rebound.Close())
			assert.NoError(t, stopProcessGracefully(process, exited), "a repeated stop must succeed")
		})
	}
}

// The fixture is a separate test process, with another descendant owning the
// port. It exercises cmd.exe -> parent -> server without requiring Python.
func TestWindowsProcessTreeHelper(t *testing.T) {
	switch os.Getenv("GOMANDER_PROCESS_TREE_HELPER") {
	case "parent":
		executable, err := os.Executable()
		require.NoError(t, err)
		child := exec.Command(executable, "-test.run=TestWindowsProcessTreeHelper")
		for _, variable := range os.Environ() {
			if !strings.HasPrefix(variable, "GOMANDER_PROCESS_TREE_HELPER=") {
				child.Env = append(child.Env, variable)
			}
		}
		child.Env = append(child.Env, "GOMANDER_PROCESS_TREE_HELPER=server")
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		require.NoError(t, child.Run())
	case "server":
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		require.NoError(t, err)
		defer listener.Close()
		// Bound the fixture lifetime even if the test's cleanup fails.
		timer := time.AfterFunc(45*time.Second, func() { _ = listener.Close() })
		defer timer.Stop()
		fmt.Printf("GOMANDER_LISTENER:%d:%s\n", os.Getpid(), listener.Addr())
		_ = http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "ready")
		}))
	}
}

func runAndReadProcess(process commandProcess) (string, error) {
	readers, err := process.Start()
	if err != nil {
		return "", err
	}

	outputs := make(chan string, len(readers))
	for _, reader := range readers {
		go func(reader io.ReadCloser) {
			output, _ := io.ReadAll(reader)
			_ = reader.Close()
			outputs <- string(output)
		}(reader)
	}

	waitErr := process.Wait()
	var output strings.Builder
	for range readers {
		output.WriteString(<-outputs)
	}
	return output.String(), waitErr
}
