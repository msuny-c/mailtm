package mailtm

import (
	"bytes"
	"encoding/json"
	"io"
    "math/rand"
	"net/http"
    "time"
)

const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

func generateString(length int) string {
    var seededRand *rand.Rand = rand.New(
    rand.NewSource(time.Now().UnixNano()))
    b := make([]byte, length)
    for i := range b {
        b[i] = charset[seededRand.Intn(len(charset))]
    }
    return string(b)
}

func makeRequest(method string, URI string, data map[string]string, token string) ([]byte, int, error) {
    var body []byte
    if data != nil {
        var err error
        body, err = json.Marshal(data)
        if err != nil {
            return nil, 0, err
        }
    }
    request, err := http.NewRequest(method, URI, bytes.NewBuffer(body))
    request.Header.Set("Content-Type", "application/json")
    if err != nil {
        return nil, 0, err
    }
    if token != "" {
        request.Header.Add("Authorization", "Bearer " + token)
    }
    client := new(http.Client)
    response, err := client.Do(request)
    if err != nil {
        return nil, 0, err
    }
    if response.StatusCode == 429 {
        time.Sleep(1 * time.Second)
        return makeRequest(method, URI, data, token)
    }
    resBody, err := io.ReadAll(response.Body)
    defer response.Body.Close()
    if err != nil {
        return nil, 0, err
    }
    return resBody, response.StatusCode, nil
}