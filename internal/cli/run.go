package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"syscall"

	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	result := &cobra.Command{
		Use:   "run-cmd",
		Short: "run a command defined in the Tiltfile",
		Run:   runCmd,
	}

	result.Flags().IntVar(&webPort, "port", DefaultWebPort, "Port for the Tilt HTTP server")
	return result
}

func runCmd(cmd *cobra.Command, args []string) {
	if len(args) < 2 {
		cmdFail(fmt.Errorf("run-cmd requires at least 2 args; got %q", args))
	}

	cmdID, podResourceID, extraArgs := args[0], args[1], args[2:]

	cmdText, err := getCmd(cmdID)
	if err != nil {
		cmdFail(fmt.Errorf("couldn't get cmd %q: %v", cmdID, err))
	}

	podID, err := getPod(podResourceID)
	if err != nil {
		cmdFail(fmt.Errorf("couldn't get pod %q: %v", podResourceID, err))
	}

	fmt.Fprintf(os.Stderr, "cli_resource %q -> %q\n", cmdID, cmdText)
	fmt.Fprintf(os.Stderr, "pod for %q -> %q\n", podResourceID, podID)

	argv := []string{"sh", "-c", cmdText, cmdID, podID}
	argv = append(argv, extraArgs...)
	argv0 := argv[0]
	argv0, err = exec.LookPath(argv0)
	if err != nil {
		cmdFail(fmt.Errorf("couldn't find binary on path: %q %v", argv0, err))
	}
	envp := os.Environ()
	fmt.Fprintf(os.Stderr, "== Tilt going away; exec'ing %q ==\n", argv)
	err = syscall.Exec(argv0, argv, envp)

	// We should never get here, because our process has been taken over by the new process
	if err != nil {
		cmdFail(fmt.Errorf("Error exec'ing %q %q %q: %v", argv0, argv, envp, err))
	}

	fmt.Fprintf(os.Stderr, "goodbye world\n")
}

func getCmd(resourceID string) (string, error) {
	apiURL := fmt.Sprintf("http://localhost:%d/api/get/cli", webPort)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("error construct request: %v", err)
	}
	q := url.Values{}
	q.Add("resource_id", resourceID)
	client := &http.Client{}

	req.URL.RawQuery = q.Encode()
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error talking to Tilt at %s: %v", apiURL, err)
	}

	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error from Tilt at %s: %v", apiURL, err)
	}

	var result map[string]string
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&result); err != nil {
		return "", fmt.Errorf("error decoding json: %v", err)
	}
	if _, ok := result["cmd"]; !ok {
		return "", fmt.Errorf("result from Tilt server missing pod_id: %v", result)
	}
	return result["cmd"], nil
}
