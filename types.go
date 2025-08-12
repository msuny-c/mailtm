package mailtm

import "time"

type Collection[T any] struct {
	Member     []T        `json:"hydra:member"`
	TotalItems int        `json:"hydra:totalItems"`
	View       *HydraView `json:"hydra:view,omitempty"`
	Search     any        `json:"hydra:search,omitempty"`
}

type HydraView struct {
	First    string `json:"hydra:first,omitempty"`
	Last     string `json:"hydra:last,omitempty"`
	Previous string `json:"hydra:previous,omitempty"`
	Next     string `json:"hydra:next,omitempty"`
}

type Domain struct {
	ID        string    `json:"id"`
	Domain    string    `json:"domain"`
	IsActive  bool      `json:"isActive"`
	IsPrivate bool      `json:"isPrivate"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Account struct {
	ID         string    `json:"id"`
	Address    string    `json:"address"`
	Quota      int       `json:"quota"`
	Used       int       `json:"used"`
	IsDisabled bool      `json:"isDisabled"`
	IsDeleted  bool      `json:"isDeleted"`
	CreatedAt  time.Time `json:"createdAt"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type Token struct {
	ID    string `json:"id"`
	Token string `json:"token"`
}

type EmailContact struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type Attachment struct {
	ID               string `json:"id"`
	Filename         string `json:"filename"`
	ContentType      string `json:"contentType"`
	Disposition      string `json:"disposition"`
	TransferEncoding string `json:"transferEncoding"`
	Related          bool   `json:"related"`
	Size             int    `json:"size"`
	DownloadURL      string `json:"downloadUrl"`
}

type Message struct {
	ID             string         `json:"id"`
	AccountID      string         `json:"accountId"`
	MsgID          string         `json:"msgid"`
	From           EmailContact   `json:"from"`
	To             []EmailContact `json:"to"`
	CC             []string       `json:"cc"`
	BCC            []string       `json:"bcc"`
	Subject        string         `json:"subject"`
	Intro          string         `json:"intro"`
	Seen           bool           `json:"seen"`
	Flagged        bool           `json:"flagged"`
	IsDeleted      bool           `json:"isDeleted"`
	Verifications  []string       `json:"verifications"`
	Retention      bool           `json:"retention"`
	RetentionDate  *time.Time     `json:"retentionDate,omitempty"`
	Text           string         `json:"text"`
	HTML           []string       `json:"html"`
	HasAttachments bool           `json:"hasAttachments"`
	Attachments    []Attachment   `json:"attachments"`
	Size           int            `json:"size"`
	DownloadURL    string         `json:"downloadUrl"`
	CreatedAt      time.Time      `json:"createdAt"`
	UpdatedAt      time.Time      `json:"updatedAt"`
}

type Source struct {
	ID          string `json:"id"`
	DownloadURL string `json:"downloadUrl"`
	Data        string `json:"data"`
}
