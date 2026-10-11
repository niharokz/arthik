// Package ai asks a local Ollama model to categorise inbox drafts. It only ever
// picks from your own category list; amounts, accounts and dates never come from
// the model.
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client talks to Ollama's /api/chat.
type Client struct {
	URL   string // e.g. http://ollama:11434
	Model string
	HTTP  *http.Client
}

// New returns a client with the given per-request timeout.
func New(url, model string, timeout time.Duration) *Client {
	return &Client{URL: strings.TrimRight(url, "/"), Model: model, HTTP: &http.Client{Timeout: timeout}}
}

// Choice is one category the model may pick.
type Choice struct {
	ID   string
	Name string // "Food › Groceries"
}

// Example is how you categorised something before (teaches the model your style).
type Example struct {
	Title    string
	Category string // category id
}

// Request describes one draft.
type Request struct {
	Kind     string // expense | income
	Amount   string
	Merchant string
	Text     string // the original message, if any
	Choices  []Choice
	Examples []Example
}

// Answer is the model's pick. Category is "" when it is unsure or picked
// something not on the list.
type Answer struct {
	Category string
	Title    string
}

const system = `You categorise personal bank transactions from India for a budgeting app.
Reply with JSON only, exactly: {"category": "<id>", "title": "<name>"}
- "category" must be one of the ids listed, or "" if none fits well.
- "title" is a short, clean name for the merchant or person (at most 4 words, normal capitalisation, no card numbers or reference codes).`

// Categorise asks the model to pick a category and a clean title.
func (c *Client) Categorise(ctx context.Context, r Request) (Answer, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "Transaction: %s of ₹%s\nMerchant text: %s\n", r.Kind, r.Amount, r.Merchant)
	if r.Text != "" {
		fmt.Fprintf(&b, "Message: %s\n", r.Text)
	}
	b.WriteString("\nCategories (id: name):\n")
	for _, ch := range r.Choices {
		fmt.Fprintf(&b, "- %s: %s\n", ch.ID, ch.Name)
	}
	if len(r.Examples) > 0 {
		b.WriteString("\nHow I categorised earlier transactions (title → id):\n")
		for _, e := range r.Examples {
			fmt.Fprintf(&b, "- %s → %s\n", e.Title, e.Category)
		}
	}
	body, _ := json.Marshal(map[string]any{
		"model":    c.Model,
		"stream":   false,
		"format":   "json",
		"options":  map[string]any{"temperature": 0},
		"messages": []map[string]string{{"role": "system", "content": system}, {"role": "user", "content": b.String()}},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.URL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return Answer{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return Answer{}, fmt.Errorf("ollama: %w", err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode != http.StatusOK {
		return Answer{}, fmt.Errorf("ollama: %s: %s", res.Status, strings.TrimSpace(string(raw)))
	}
	var out struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return Answer{}, fmt.Errorf("ollama: unreadable reply: %w", err)
	}
	var a struct {
		Category string `json:"category"`
		Title    string `json:"title"`
	}
	if err := json.Unmarshal([]byte(extractJSON(out.Message.Content)), &a); err != nil {
		return Answer{}, fmt.Errorf("ollama: reply is not the JSON asked for: %q", out.Message.Content)
	}
	ans := Answer{Title: strings.TrimSpace(a.Title)}
	for _, ch := range r.Choices {
		if ch.ID == strings.TrimSpace(a.Category) {
			ans.Category = ch.ID
		}
	}
	if len([]rune(ans.Title)) > 40 {
		ans.Title = ""
	}
	return ans, nil
}

// extractJSON trims anything around the first {...} (some models add prose).
func extractJSON(s string) string {
	i, j := strings.Index(s, "{"), strings.LastIndex(s, "}")
	if i < 0 || j < i {
		return s
	}
	return s[i : j+1]
}
