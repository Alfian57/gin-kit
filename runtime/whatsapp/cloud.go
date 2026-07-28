package whatsapp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
)

func (c *cloudClient) endpoint() string {
	return c.options.APIBaseURL + "/" + c.options.APIVersion + "/" + url.PathEscape(c.options.PhoneNumberID) + "/messages"
}

func encodePayload(recipient string, message *Message) (*bytes.Reader, error) {
	payload := struct {
		MessagingProduct string `json:"messaging_product"`
		To               string `json:"to"`
		Type             string `json:"type"`
		Template         struct {
			Name     string `json:"name"`
			Language struct {
				Code string `json:"code"`
			} `json:"language"`
			Components []TemplateComponent `json:"components,omitempty"`
		} `json:"template"`
	}{
		MessagingProduct: "whatsapp",
		To:               recipient,
		Type:             "template",
	}
	payload.Template.Name = strings.TrimSpace(message.template)
	payload.Template.Language.Code = strings.TrimSpace(message.language)
	payload.Template.Components = message.components
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("whatsapp: encode template message: %w", err)
	}
	return bytes.NewReader(encoded), nil
}
