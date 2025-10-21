package ollama

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"strings"
	"time"

	"github.com/projectdiscovery/gologger"
	"github.com/valyala/fasthttp"
)

// Chat API structs
type ChatMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

type ChatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

// Model validation structs
type ModelDetails struct {
	ParentModel       string   `json:"parent_model"`
	Format            string   `json:"format"`
	Family            string   `json:"family"`
	Families          []string `json:"families"`
	ParameterSize     string   `json:"parameter_size"`
	QuantizationLevel string   `json:"quantization_level"`
}

type Model struct {
	Name       string       `json:"name"`
	Model      string       `json:"model"`
	ModifiedAt string       `json:"modified_at"`
	Size       int64        `json:"size"`
	Digest     string       `json:"digest"`
	Details    ModelDetails `json:"details"`
}

type ModelsResponse struct {
	Models []Model `json:"models"`
}

type Client struct {
	Host        string
	Model       string
	ChatMessage ChatMessage
	Timeout     time.Duration
	HTTPClient  *fasthttp.Client
}

func NewClient(host, model string, timeout time.Duration) *Client {
	return &Client{
		Host:       host,
		Model:      model,
		Timeout:    timeout,
		HTTPClient: &fasthttp.Client{ReadTimeout: timeout, WriteTimeout: timeout},
	}
}

type Result struct {
	URL   string
	Match bool
	Err   error
}

// CheckModelExists validates if the specified model is available in Ollama
func (o *Client) CheckModelExists(debug bool) error {
	if debug {
		gologger.Debug().Msgf("Checking if model '%s' exists in Ollama", o.Model)
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)

	req.SetRequestURI(o.Host + "/api/tags")
	req.Header.SetMethod("GET")

	if err := o.HTTPClient.DoTimeout(req, resp, o.Timeout); err != nil {
		if debug {
			gologger.Debug().Msgf("Failed to connect to Ollama API at %s: %v", o.Host, err)
		}
		return fmt.Errorf("failed to connect to Ollama API: %v", err)
	}

	if resp.StatusCode() != 200 {
		if debug {
			gologger.Debug().Msgf("Received status %d from /api/tags", resp.StatusCode())
		}
		return fmt.Errorf("Ollama API returned status %d", resp.StatusCode())
	}

	var modelsResp ModelsResponse
	if err := json.Unmarshal(resp.Body(), &modelsResp); err != nil {
		if debug {
			gologger.Debug().Msgf("Failed to parse models response: %v", err)
		}
		return fmt.Errorf("failed to parse models response: %v", err)
	}

	// Check if the specified model exists
	modelFound := false
	availableModels := make([]string, 0, len(modelsResp.Models))

	for _, model := range modelsResp.Models {
		availableModels = append(availableModels, model.Name)
		// Check both exact name match and name without :latest suffix
		if model.Name == o.Model ||
			(strings.HasSuffix(model.Name, ":latest") && strings.TrimSuffix(model.Name, ":latest") == o.Model) ||
			(strings.HasSuffix(o.Model, ":latest") && strings.TrimSuffix(o.Model, ":latest") == strings.TrimSuffix(model.Name, ":latest")) {
			modelFound = true
			if debug {
				gologger.Debug().Msgf("Found model: %s (size: %d bytes, family: %s)", model.Name, model.Size, model.Details.Family)
			}
			break
		}
	}

	if !modelFound {
		if debug {
			gologger.Debug().Msgf("Model '%s' not found. Available models: %v", o.Model, availableModels)
		}
		return fmt.Errorf("model '%s' not found. Available models: %v", o.Model, availableModels)
	}

	if debug {
		gologger.Debug().Msgf("Model '%s' is available", o.Model)
	}
	return nil
}

// Download favicon from URL and return base64-encoded string
func (o *Client) DownloadImageAsBase64(url string, debug bool) (string, error) {
	if debug {
		gologger.Debug().Msgf("Downloading image from: %s", url)
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)
	req.SetRequestURI(url)
	req.Header.SetMethod("GET")
	if err := o.HTTPClient.DoTimeout(req, resp, o.Timeout); err != nil {
		if debug {
			gologger.Debug().Msgf("Failed to fetch %s: %v", url, err)
		}
		return "", fmt.Errorf("error fetching %s: %v", url, err)
	}

	if resp.StatusCode() != 200 {
		if debug {
			gologger.Debug().Msgf("Bad status code for %s: %d", url, resp.StatusCode())
		}
		return "", fmt.Errorf("bad status for %s: %d", url, resp.StatusCode())
	}

	// Read image bytes
	data := resp.Body()
	if debug {
		gologger.Debug().Msgf("Downloaded %d bytes from %s", len(data), url)
	}

	// Decode image to check format
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		if debug {
			gologger.Debug().Msgf("Failed to decode image from %s: %v", url, err)
		}
		return "", fmt.Errorf("error decoding image from %s: %v", url, err)
	}

	if debug {
		gologger.Debug().Msgf("Decoded image format: %s, dimensions: %dx%d", format, img.Bounds().Dx(), img.Bounds().Dy())
	}

	// Encode as PNG (more universally supported for APIs)
	var buf bytes.Buffer
	if format == "png" {
		buf.Write(data) // already PNG, just reuse bytes
		if debug {
			gologger.Debug().Msgf("Image already in PNG format, reusing bytes")
		}
	} else {
		if err := png.Encode(&buf, img); err != nil {
			if debug {
				gologger.Debug().Msgf("Failed to encode PNG: %v", err)
			}
			return "", fmt.Errorf("error encoding PNG: %v", err)
		}
		if debug {
			gologger.Debug().Msgf("Converted %s to PNG format", format)
		}
	}

	// Convert to base64
	b64 := base64.StdEncoding.EncodeToString(buf.Bytes())
	if debug {
		gologger.Debug().Msgf("Generated base64 string of length: %d", len(b64))
	}
	return b64, nil
}

// Compare two favicons using Ollama chat API
func (o *Client) CompareFaviconsChatAPI(base64Base, base64Target string, debug bool) (bool, error) {
	if debug {
		gologger.Debug().Msgf("Starting comparison with model: %s", o.Model)
	}

	reqBody := ChatRequest{
		Model: o.Model,
		Messages: []ChatMessage{
			{
				Role:    "user",
				Content: "Compare these two favicons. Respond only with Yes if visually identical or same brand/logo, otherwise No.",
				Images:  []string{base64Base, base64Target},
			},
		},
		Stream: true,
	}

	body, _ := json.Marshal(reqBody)
	if debug {
		gologger.Debug().Msgf("Sending request to Ollama API, payload size: %d bytes", len(body))
	}

	req := fasthttp.AcquireRequest()
	resp := fasthttp.AcquireResponse()
	defer fasthttp.ReleaseRequest(req)
	defer fasthttp.ReleaseResponse(resp)
	req.SetRequestURI(o.Host + "/api/chat")
	req.Header.SetMethod("POST")
	req.Header.SetContentType("application/json")
	req.SetBody(body)
	if err := o.HTTPClient.DoTimeout(req, resp, o.Timeout); err != nil {
		if debug {
			gologger.Debug().Msgf("Failed to connect to Ollama API at %s: %v", o.Host, err)
		}
		return false, err
	}

	if debug {
		gologger.Debug().Msgf("Received response from Ollama, status: %d", resp.StatusCode())
	}

	responseText := string(resp.Body())
	lines := strings.Split(responseText, "\n")
	var fullText strings.Builder
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var chunk map[string]any
		if err := json.Unmarshal([]byte(line), &chunk); err != nil {
			if debug {
				gologger.Debug().Msgf("Failed to parse JSON chunk: %v", err)
			}
			continue
		}
		if msg, ok := chunk["message"].(map[string]any); ok {
			if content, ok := msg["content"].(string); ok {
				fullText.WriteString(content)
			}
		}
		if done, ok := chunk["done"].(bool); ok && done {
			if debug {
				gologger.Debug().Msgf("Streaming response complete")
			}
			break
		}
	}

	answer := fullText.String()
	if debug {
		gologger.Debug().Msgf("Model response: %s", answer)
	}

	match := strings.Contains(answer, "Yes")
	if debug {
		gologger.Debug().Msgf("Match result: %v", match)
	}

	return match, nil
}
