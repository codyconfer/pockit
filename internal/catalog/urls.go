package catalog

import (
	"fmt"
	"net/url"
	"strings"
)

func CheckURLSecrets(raw string) error {
	u, e := url.Parse(raw)
	if e != nil {
		return fmt.Errorf("invalid source URL (value withheld)")
	}
	if u.User != nil {
		return fmt.Errorf("credentials in source URL refused")
	}
	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return fmt.Errorf("invalid source query (value withheld)")
	}
	for key := range query {
		k := strings.NewReplacer("-", "", "_", "", ".", "").Replace(strings.ToLower(key))
		if strings.Contains(k, "token") || strings.Contains(k, "secret") || strings.Contains(k, "password") || strings.Contains(k, "credential") || strings.Contains(k, "signature") || strings.Contains(k, "apikey") || k == "key" || k == "auth" || k == "authorization" || k == "sig" {
			return fmt.Errorf("credential-bearing source query refused")
		}
	}
	return nil
}
func checkURLs(v any) error {
	switch value := v.(type) {
	case map[string]any:
		for k, item := range value {
			if k == "url" {
				if s, ok := item.(string); ok {
					if e := CheckURLSecrets(s); e != nil {
						return e
					}
				}
			}
			if e := checkURLs(item); e != nil {
				return e
			}
		}
	case []any:
		for _, item := range value {
			if e := checkURLs(item); e != nil {
				return e
			}
		}
	}
	return nil
}
