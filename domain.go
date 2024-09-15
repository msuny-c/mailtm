package mailtm

import (
	"time"
	"encoding/json"
)

type Domain struct {
	Id string `json:"id"`
	Domain string `json:"domain"`
	IsActive bool `json:"isActive"`
	IsPrivate bool `json:"isPrivate"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func AvailableDomains() ([]Domain, error) {
	body, code, err := makeRequest("GET", URI_DOMAINS, nil, "")
	if err != nil {
		return nil, err
	}
	if code != 200 {
		return nil, err
	}
	data := map[string][]Domain{}
	json.Unmarshal(body, &data)
	return data["hydra:member"], nil
}