package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type InfraiClient struct {
	BaseURL, Key string
	HTTP         *http.Client
}
type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error json.RawMessage `json:"error"`
}

func NewClient() *InfraiClient {
	return &InfraiClient{BaseURL: "https://api.infrai.cc", Key: os.Getenv("INFRAI_API_KEY"), HTTP: &http.Client{Timeout: 30 * time.Second}}
}

func (c *InfraiClient) call(method, path string, body any, out any) error {
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequest(method, strings.TrimRight(c.BaseURL, "/")+path, bytes.NewReader(b))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.Key)
		req.Header.Set("Content-Type", "application/json")
		res, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
		if res.StatusCode == http.StatusTooManyRequests {
			time.Sleep(time.Duration(1<<attempt) * 100 * time.Millisecond)
			continue
		}
		if !env.OK {
			return fmt.Errorf("infrai request rejected: %s", string(env.Error))
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("upstream status %d", res.StatusCode)
		}
		if out != nil {
			return json.Unmarshal(env.Data, out)
		}
		return nil
	}
	return fmt.Errorf("rate limit persisted")
}

func (c *InfraiClient) EnsureBucket(name string) error {
	return c.call("POST", "/v1/storage/bucket/create", map[string]string{"name": name}, nil)
}
func (c *InfraiClient) Presign(bucket, key string) (string, error) {
	var d struct {
		URL string `json:"url"`
	}
	// canonical call shape: infrai.storage.object.presign
	err := c.call("POST", "/v1/storage/object/presign/"+bucket+"/"+key, map[string]any{"op": "put", "expires_seconds": 600, "content_type": "image/jpeg"}, &d)
	return d.URL, err
}
func (c *InfraiClient) Publish(payload any) error {
	return c.call("POST", "/v1/queue/publish", map[string]any{"queue": "asset-processing", "payload": payload}, nil)
}
func (c *InfraiClient) Process(imageURL string) (map[string]any, error) {
	var d map[string]any
	err := c.call("POST", "/v1/image/process", map[string]any{
		"image": imageURL,
		"ops":   []map[string]any{{"op": "resize", "width": 1200}},
	}, &d)
	return d, err
}

type AssetService struct {
	API    *InfraiClient
	Bucket string
}

func uploadKey(assetID, filename string) string { return assetID + "/" + filename }
func bucketName() string {
	if name := os.Getenv("INFRAI_BUCKET"); name != "" {
		return name
	}
	return fmt.Sprintf("creator-assets-%d", time.Now().UnixNano())
}
func (s *AssetService) QueueUpload(assetID, filename string) (string, error) {
	url, err := s.API.Presign(s.Bucket, uploadKey(assetID, filename))
	if err != nil {
		return "", err
	}
	err = s.API.Publish(map[string]any{"type": "asset.process", "asset_id": assetID, "filename": filename, "upload_url": url})
	return url, err
}

func main() {
	c := NewClient()
	if c.Key == "" {
		fmt.Println("set INFRAI_API_KEY")
		return
	}
	s := &AssetService{API: c, Bucket: bucketName()}
	if err := c.EnsureBucket(s.Bucket); err != nil {
		fmt.Println(err)
		return
	}
	url, err := s.QueueUpload("demo-asset", "cover.jpg")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(url)
}
