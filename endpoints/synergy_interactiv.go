package endpoints

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

func PostbackToPartnerURL(price float64, postbackData map[string]interface{}, url string) (string, error) {

	postbackBody, err := json.Marshal(postbackData)
	if err != nil {
		return "Failed to convert postback data to JSON", err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(postbackBody))
	if err != nil {
		return "Failed to create POST request", err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "Failed to get a response from the postback request", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "Failed to read the response body", err
	}

	if resp.StatusCode != http.StatusOK {
		return strconv.Itoa(resp.StatusCode), fmt.Errorf("postback failed with status: %s", resp.Status)
	}

	return string(body), nil
}
