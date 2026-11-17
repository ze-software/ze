// Design: docs/architecture/container/image-store.md -- the registry requests: the challenge and the token
// Related: client.go -- get, which answers a challenge once per request

package registry

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/ze-software/ze/internal/core/redact"
)

// Bounds of the authentication exchange.
const (
	// tokenAnswerSizeMax bounds the realm's answer (design: 64 KiB).
	tokenAnswerSizeMax = 64 << 10
	// challengeLengthMax bounds the challenge header that is parsed.
	challengeLengthMax = 4 << 10
	// challengeParamsMax bounds the parameters one challenge may carry.
	challengeParamsMax = 16
)

// answerChallenge reads the challenge of a 401 and sets the authorization of
// o from it: a token from the realm for a bearer challenge, the credentials
// for a basic one. The credentials go to the realm, or to the registry
// under a basic challenge, and to no other host.
func (c *Client) answerChallenge(ctx context.Context, o *origin, header, scope string) error {
	scheme, params, err := parseChallenge(header)
	if err != nil {
		return failed(FailureAuthRefused, o.host+" asked for authentication", "its challenge cannot be read: "+err.Error())
	}
	switch strings.ToLower(scheme) {
	case "bearer":
		token, err := c.token(ctx, o, params, scope)
		if err != nil {
			return err
		}
		o.authorization = "Bearer " + token
		return nil
	case "basic":
		if o.credentials.IsZero() {
			return failed(FailureAuthRefused, o.host+" asked for credentials", "the configuration holds none for this registry")
		}
		o.authorization = o.credentials.basic()
		return nil
	}
	return failed(FailureAuthRefused, o.host+" asked for authentication", "its challenge scheme "+registryErrorCode(strings.ToUpper(scheme))+" is not one Ze answers: Bearer or Basic")
}

// token asks the realm of a bearer challenge for a token, with the
// credentials when there are any. The realm is asked over HTTPS unless the
// registry's transport is HTTP, so credentials never cross plain HTTP to a
// registry the operator reaches over TLS.
func (c *Client) token(ctx context.Context, o *origin, params map[string]string, scope string) (string, error) {
	noRealm := failed(FailureAuthRefused, o.host+" asked for a token", "its challenge names no realm Ze can ask")
	realm, err := url.Parse(params["realm"])
	if err != nil {
		return "", noRealm
	}
	if realm.Host == "" {
		return "", noRealm
	}
	switch realm.Scheme {
	case schemeHTTPS:
	case schemeHTTP:
		if o.scheme == schemeHTTPS {
			return "", failed(FailureAuthRefused, o.host+" named the token realm "+redact.URL(realm.Scheme+"://"+realm.Host), "the registry is reached over HTTPS, and its realm would be asked without TLS")
		}
	default:
		return "", noRealm
	}
	if challenged := params["scope"]; challenged != "" {
		scope = challenged
	}
	query := realm.Query()
	if service := params["service"]; service != "" {
		query.Set("service", service)
	}
	query.Set("scope", scope)
	realm.RawQuery = query.Encode()

	authorization := ""
	if !o.credentials.IsZero() {
		authorization = o.credentials.basic()
	}
	var token string
	err = c.withRetry(ctx, func() error {
		answered, err := c.tokenOnce(ctx, realm, authorization)
		token = answered
		return err
	})
	return token, err
}

// tokenOnce makes one request to the realm and reads its token.
func (c *Client) tokenOnce(ctx context.Context, realm *url.URL, authorization string) (string, error) {
	response, err := c.follow(ctx, realm.String(), "application/json", canonicalHost(realm), authorization)
	if err != nil {
		return "", err
	}
	defer discard(response)
	if response.StatusCode != http.StatusOK {
		return "", statusFailure("the token realm "+realm.Host, response)
	}
	body, over, err := readBounded(response.Body, tokenAnswerSizeMax)
	if err != nil {
		return "", &Error{failure: FailureUnreachable, problem: "the token realm " + realm.Host + " stopped answering", why: "its answer could not be read", cause: redact.URLError(err), retry: true}
	}
	if over {
		return "", failed(FailureRefused, "the token realm "+realm.Host+" answered too much", "a token answer is at most 64 KiB")
	}
	var granted struct {
		Token       string `json:"token"`
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(body, &granted) != nil {
		return "", failed(FailureAuthRefused, "the token realm "+realm.Host+" answered", "its answer is not the JSON of a token")
	}
	token := granted.Token
	if token == "" {
		token = granted.AccessToken
	}
	if token == "" {
		return "", failed(FailureAuthRefused, "the token realm "+realm.Host+" answered", "its answer holds no token")
	}
	if !isToken68(token) {
		return "", failed(FailureAuthRefused, "the token realm "+realm.Host+" answered", "its token holds characters an authorization header cannot carry")
	}
	return token, nil
}

// isToken68 answers whether token is the token68 form a bearer header
// carries, so no character of it can end or split the header.
func isToken68(token string) bool {
	body := strings.TrimRight(token, "=")
	if body == "" {
		return false
	}
	for _, c := range body {
		if !isToken68Character(c) {
			return false
		}
	}
	return true
}

// isToken68Character answers whether c is a character of token68.
func isToken68Character(c rune) bool {
	switch {
	case isAlnum(c), c >= 'A' && c <= 'Z':
		return true
	}
	return strings.ContainsRune("-._~+/", c)
}

// parseChallenge reads `Scheme key="value", key=value` from a
// WWW-Authenticate header: the scheme of the first challenge and its
// parameters, keys in lower case.
func parseChallenge(header string) (string, map[string]string, error) {
	if len(header) > challengeLengthMax {
		return "", nil, errors.New("the challenge is longer than 4 KiB")
	}
	scheme, rest, _ := strings.Cut(strings.TrimSpace(header), " ")
	if scheme == "" {
		return "", nil, errors.New("the challenge is empty")
	}
	params := map[string]string{}
	rest = strings.TrimSpace(rest)
	for rest != "" {
		if len(params) == challengeParamsMax {
			return "", nil, errors.New("the challenge carries too many parameters")
		}
		key, after, found := strings.Cut(rest, "=")
		if !found {
			return "", nil, errors.New("a parameter has no value")
		}
		value, remaining, err := challengeValue(after)
		if err != nil {
			return "", nil, err
		}
		params[strings.ToLower(strings.TrimSpace(key))] = value
		rest = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(remaining), ","))
	}
	return scheme, params, nil
}

// challengeValue reads one parameter value, quoted or not, and answers the
// text after it.
func challengeValue(text string) (string, string, error) {
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, `"`) {
		value, rest, _ := strings.Cut(text, ",")
		return strings.TrimSpace(value), rest, nil
	}
	var value strings.Builder
	for i := 1; i < len(text); i++ {
		switch text[i] {
		case '\\':
			i++
			if i < len(text) {
				value.WriteByte(text[i])
			}
		case '"':
			return value.String(), text[i+1:], nil
		default:
			value.WriteByte(text[i])
		}
	}
	return "", "", errors.New("a quoted value is not closed")
}
