// Design: docs/research/vpp-deployment-reference.md -- registered runtime prerequisites
// Related: startup_feature.go -- immutable requirements supplied by feature owners
// GoVPP v0.13.0: https://github.com/FDio/govpp/blob/v0.13.0/core/channel.go
// CheckCompatiblity reads the connected VPP's message-name/CRC table. A plugin
// supplies that table from its VLIB_API_INIT_FUNCTION, so this checks loaded
// APIs, including patched layouts, rather than copies installed on disk.

package vpp

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"go.fd.io/govpp/api"

	"github.com/ze-software/ze/internal/core/naming"
	"github.com/ze-software/ze/internal/core/opererr"
)

type startupRuntime struct {
	messages []api.Message
	code     string
	fix      string
}

// startupAPIMessage is metadata only, never a message that can be sent. Its
// value fields make installed startup requirements stable equality values.
type startupAPIMessage struct {
	name string
	crc  string
	kind api.MessageType
}

func (m startupAPIMessage) GetMessageName() string          { return m.name }
func (m startupAPIMessage) GetCrcString() string            { return m.crc }
func (m startupAPIMessage) GetMessageType() api.MessageType { return m.kind }

func newStartupRuntime(input *StartupRuntime) (*startupRuntime, error) {
	if input == nil {
		return nil, nil //nolint:nilnil // An omitted optional declaration has no runtime API requirements.
	}
	if len(input.Messages) == 0 {
		return nil, errors.New("VPP runtime requirement has no binary API messages")
	}
	if err := naming.ValidateNodeName("VPP runtime diagnostic code", input.Code, 127); err != nil {
		return nil, err
	}
	if strings.TrimSpace(input.Fix) == "" {
		return nil, errors.New("VPP runtime requirement has no repair command")
	}
	result := &startupRuntime{code: input.Code, fix: input.Fix, messages: make([]api.Message, 0, len(input.Messages))}
	for _, message := range input.Messages {
		if message == nil {
			return nil, errors.New("VPP runtime requirement contains a nil message")
		}
		name, crc := message.GetMessageName(), message.GetCrcString()
		if err := naming.ValidateNodeName("VPP binary API message", name, 127); err != nil {
			return nil, err
		}
		if len(crc) != 8 {
			return nil, fmt.Errorf("VPP binary API %s must declare an eight-digit CRC", name)
		}
		if _, err := hex.DecodeString(crc); err != nil {
			return nil, fmt.Errorf("VPP binary API %s has an invalid CRC: %w", name, err)
		}
		result.messages = append(result.messages, startupAPIMessage{name: name, crc: crc, kind: message.GetMessageType()})
	}
	return result, nil
}

// checkStartupRuntime MUST complete before the manager publishes readiness.
// The manager MUST close a refused connection and its runtime statistics before
// retrying. No native state is mutated by this compatibility check.
func (s *VPPSettings) checkStartupRuntime(ctx context.Context, connector *Connector) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !s.needsStartupRuntime() {
		return nil
	}
	if connector == nil {
		return startupProbeError(errors.New("VPP API connection is unavailable"))
	}
	generation, err := connector.ConnectionGeneration()
	if err != nil {
		return startupProbeError(err)
	}
	channel, err := connector.NewChannel()
	if err != nil {
		return startupProbeError(err)
	}
	defer channel.Close()
	err = s.checkStartupAPI(ctx, channel)
	current, generationErr := connector.ConnectionGeneration()
	if generationErr != nil {
		return startupProbeError(errors.Join(err, generationErr))
	}
	if current != generation {
		return startupProbeError(errors.Join(err, errVppGenerationChanged))
	}
	return err
}

func (s *VPPSettings) needsStartupRuntime() bool {
	for _, feature := range s.startup {
		if feature.runtime != nil {
			return true
		}
	}
	return false
}

func (s *VPPSettings) checkStartupAPI(ctx context.Context, channel api.Channel) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, feature := range s.startup {
		if feature.runtime == nil {
			continue
		}
		if err := channel.CheckCompatiblity(feature.runtime.messages...); err != nil {
			return opererr.Wrap(err, "VPP plugin "+feature.plugin+" does not provide its required binary APIs",
				"The running VPP must load the plugin with the same message layouts as this Ze build.", feature.runtime.fix)
		}
		if err := ctx.Err(); err != nil {
			return err
		}
	}
	return nil
}

func startupProbeError(err error) error {
	return opererr.Wrap(err, "VPP plugin readiness could not be checked",
		"An unavailable API connection does not show whether its plugins are loaded.", "ze doctor")
}
