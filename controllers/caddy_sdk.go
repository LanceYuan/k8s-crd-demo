package controllers

import (
	"bytes"
	"encoding/json"
	devopsv1 "k8s-crd-demo/api/v1"
	"net/http"
	"time"
)

func AddCaddyRoute(app *devopsv1.App) error {
	client := http.Client{Timeout: 10 * time.Second}
	body := map[string]interface{}{
		"match": []map[string]interface{}{
			{
				"path": []string{app.Spec.Path},
			},
		},
		"handle": []map[string]interface{}{
			{
				"body":    app.Spec.Content,
				"handler": "static_response",
			},
		},
	}
	bodyByte, err := json.Marshal(body)
	if err != nil {
		return err
	}
	reader := bytes.NewReader(bodyByte)
	req, err := http.NewRequest(http.MethodPost, "http://caddy-controller.codepy.net/config/apps/http/servers/srv0/routes", reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	_, err = client.Do(req)
	if err != nil {
		return err
	}
	return nil
}
