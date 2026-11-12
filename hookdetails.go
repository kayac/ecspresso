package ecspresso

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/document"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// DeploymentLifecycleHook.HookDetails is a document.Interface of the AWS SDK,
// which encoding/json can neither decode nor encode. So hookDetails is taken
// out of the service definition before decoding it, and filled in after
// encoding it.

// lookupFold finds a key in m case-insensitively, as encoding/json does for struct fields.
func lookupFold(m map[string]any, name string) (string, any) {
	for k, v := range m {
		if strings.EqualFold(k, name) {
			return k, v
		}
	}
	return "", nil
}

// extractHookDetails removes hookDetails of the lifecycle hooks from src,
// and returns them by the index of the hook.
// src is returned as it is when it has no hookDetails.
func extractHookDetails(src []byte) ([]byte, map[int]any, error) {
	var m map[string]any
	if err := json.Unmarshal(src, &m); err != nil {
		// leave it to the decoder of the service definition to report the error
		return src, nil, nil
	}
	_, dc := lookupFold(m, "deploymentConfiguration")
	dcm, _ := dc.(map[string]any)
	_, hs := lookupFold(dcm, "lifecycleHooks")
	hooks, _ := hs.([]any)
	details := map[int]any{}
	for i, h := range hooks {
		hook, _ := h.(map[string]any)
		if k, v := lookupFold(hook, "hookDetails"); v != nil {
			details[i] = v
			delete(hook, k)
		}
	}
	if len(details) == 0 {
		return src, nil, nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return nil, nil, err
	}
	return b, details, nil
}

func setHookDetails(sv *Service, details map[int]any) {
	if sv.DeploymentConfiguration == nil {
		return
	}
	hooks := sv.DeploymentConfiguration.LifecycleHooks
	for i, v := range details {
		if i < len(hooks) {
			hooks[i].HookDetails = document.NewLazyDocument(v)
		}
	}
}

func lifecycleHooksOf(v any) []types.DeploymentLifecycleHook {
	var dc *types.DeploymentConfiguration
	switch v := v.(type) {
	case *Service:
		if v != nil {
			dc = v.DeploymentConfiguration
		}
	case *ServiceForDiff:
		if v != nil && v.UpdateServiceInput != nil {
			dc = v.DeploymentConfiguration
		}
	case *ecs.UpdateServiceInput:
		if v != nil {
			dc = v.DeploymentConfiguration
		}
	}
	if dc == nil {
		return nil
	}
	return dc.LifecycleHooks
}

// fillHookDetails sets hookDetails of the lifecycle hooks of v into m, which is v encoded for API.
// This must run after rewriting the keys of m to keep the keys in hookDetails as they are.
func fillHookDetails(v any, m map[string]any) error {
	hooks := lifecycleHooksOf(v)
	if len(hooks) == 0 {
		return nil
	}
	dcm, _ := m["deploymentConfiguration"].(map[string]any)
	hms, _ := dcm["lifecycleHooks"].([]any)
	for i, h := range hooks {
		if i >= len(hms) {
			break
		}
		hm, ok := hms[i].(map[string]any)
		if !ok {
			continue
		}
		delete(hm, "hookDetails")
		if h.HookDetails == nil {
			continue
		}
		b, err := h.HookDetails.MarshalSmithyDocument()
		if err != nil {
			return fmt.Errorf("failed to marshal hookDetails of lifecycleHooks[%d]: %w", i, err)
		}
		var d any
		if err := json.Unmarshal(b, &d); err != nil {
			return fmt.Errorf("failed to marshal hookDetails of lifecycleHooks[%d]: %w", i, err)
		}
		if d != nil {
			hm["hookDetails"] = d
		}
	}
	return nil
}
