package cli

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/spf13/cobra"
)

func newGetCmd() *cobra.Command {
	result := &cobra.Command{
		Use:   "get",
		Short: "get state from running Tilt",
	}

	result.AddCommand(newGetPodCmd())
	return result
}

func newGetPodCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pod",
		Short: "get the pod ID for a resource",
		Run:   getPodCLI,
	}
	cmd.Flags().IntVar(&webPort, "port", DefaultWebPort, "Port for the Tilt HTTP server")
	return cmd
}

func getPodCLI(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		cmdFail(fmt.Errorf("get pod takes exactly 1 arg; got %q", args))
	}

	podID, err := getPod(args[0])
	if err != nil {
		cmdFail(err)
	}
	fmt.Printf("%s\n", podID)
}

func getPod(resourceID string) (string, error) {
	apiURL := fmt.Sprintf("http://localhost:%d/api/get/pod", webPort)
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
	if _, ok := result["pod_id"]; !ok {
		return "", fmt.Errorf("result from Tilt server missing pod_id: %v", result)
	}
	return result["pod_id"], nil
}
