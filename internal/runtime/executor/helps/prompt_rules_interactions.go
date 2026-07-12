package helps

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// interactionsPromptFmt mutates native Google Interactions requests before translation.
type interactionsPromptFmt struct{}

type interactionsTextTarget struct {
	path         string
	array        bool
	isTextBlock  func(gjson.Result) bool
	newTextBlock func(string) ([]byte, error)
}

func (interactionsPromptFmt) InjectSystem(payload []byte, content, marker, position string) []byte {
	if content == "" {
		return payload
	}
	target, ok := interactionsSystemTarget(payload)
	if !ok {
		if marker != "" {
			return payload
		}
		updated, err := sjson.SetBytes(payload, "system_instruction", content)
		if err != nil {
			return payload
		}
		return updated
	}
	return injectInteractionsTarget(payload, target, content, marker, position)
}

func (interactionsPromptFmt) StripSystem(payload []byte, re *regexp.Regexp) []byte {
	target, ok := interactionsSystemTarget(payload)
	if !ok {
		return payload
	}
	return stripInteractionsTarget(payload, target, re)
}

func (interactionsPromptFmt) InjectLastUser(payload []byte, content, marker, position string) []byte {
	if content == "" {
		return payload
	}
	target, ok := interactionsLastUserTarget(payload)
	if !ok {
		return payload
	}
	return injectInteractionsTarget(payload, target, content, marker, position)
}

func (interactionsPromptFmt) StripLastUser(payload []byte, re *regexp.Regexp) []byte {
	target, ok := interactionsLastUserTarget(payload)
	if !ok {
		return payload
	}
	return stripInteractionsTarget(payload, target, re)
}

func interactionsSystemTarget(payload []byte) (interactionsTextTarget, bool) {
	system := gjson.GetBytes(payload, "system_instruction")
	if !system.Exists() {
		return interactionsTextTarget{}, false
	}
	if system.Type == gjson.String {
		return interactionsTextTarget{path: "system_instruction"}, true
	}
	if text := system.Get("text"); text.Exists() && text.Type == gjson.String {
		return interactionsTextTarget{path: "system_instruction.text"}, true
	}
	if parts := system.Get("parts"); parts.IsArray() && interactionsArrayHasText(parts, isInteractionsNativeTextBlock) {
		return interactionsTextTarget{
			path:         "system_instruction.parts",
			array:        true,
			isTextBlock:  isInteractionsNativeTextBlock,
			newTextBlock: newInteractionsNativeTextBlock,
		}, true
	}
	return interactionsTextTarget{}, false
}

func interactionsLastUserTarget(payload []byte) (interactionsTextTarget, bool) {
	input := gjson.GetBytes(payload, "input")
	if !input.Exists() {
		return interactionsTextTarget{}, false
	}
	if input.Type == gjson.String {
		return interactionsTextTarget{path: "input"}, strings.TrimSpace(input.String()) != ""
	}
	targets := make([]interactionsTextTarget, 0)
	collectInteractionsInputTargets(input, "input", "user", &targets)
	if len(targets) == 0 {
		return interactionsTextTarget{}, false
	}
	return targets[len(targets)-1], true
}

func collectInteractionsInputTargets(value gjson.Result, path, defaultRole string, targets *[]interactionsTextTarget) {
	if value.Type == gjson.String {
		if defaultRole == "user" && strings.TrimSpace(value.String()) != "" {
			*targets = append(*targets, interactionsTextTarget{path: path})
		}
		return
	}
	if value.IsArray() {
		for i, item := range value.Array() {
			collectInteractionsInputItemTargets(item, fmt.Sprintf("%s.%d", path, i), defaultRole, targets)
		}
		return
	}
	if steps := value.Get("steps"); steps.IsArray() {
		role := interactionsRole(value.Get("role").String(), defaultRole)
		for i, step := range steps.Array() {
			collectInteractionsInputItemTargets(step, fmt.Sprintf("%s.steps.%d", path, i), role, targets)
		}
		return
	}
	collectInteractionsInputItemTargets(value, path, defaultRole, targets)
}

func collectInteractionsInputItemTargets(item gjson.Result, path, defaultRole string, targets *[]interactionsTextTarget) {
	if item.Type == gjson.String {
		collectInteractionsInputTargets(item, path, defaultRole, targets)
		return
	}
	if steps := item.Get("steps"); steps.IsArray() {
		role := interactionsRole(item.Get("role").String(), defaultRole)
		for i, step := range steps.Array() {
			collectInteractionsInputItemTargets(step, fmt.Sprintf("%s.steps.%d", path, i), role, targets)
		}
		return
	}

	stepType := strings.ToLower(strings.TrimSpace(item.Get("type").String()))
	role := defaultRole
	switch stepType {
	case "model_output", "thought", "function_call":
		return
	case "function_result":
		return
	case "user_input":
		role = "user"
	case "":
		role = interactionsRole(item.Get("role").String(), defaultRole)
	default:
		role = interactionsRole(item.Get("role").String(), defaultRole)
	}
	if role != "user" {
		return
	}

	if parts := item.Get("parts"); parts.IsArray() {
		if interactionsArrayHasText(parts, isInteractionsNativeTextBlock) {
			*targets = append(*targets, interactionsTextTarget{
				path:         path + ".parts",
				array:        true,
				isTextBlock:  isInteractionsNativeTextBlock,
				newTextBlock: newInteractionsNativeTextBlock,
			})
		}
		return
	}

	content := item.Get("content")
	if content.Type == gjson.String && strings.TrimSpace(content.String()) != "" {
		*targets = append(*targets, interactionsTextTarget{path: path + ".content"})
		return
	}
	if content.IsArray() && interactionsArrayHasText(content, isInteractionsContentTextBlock) {
		*targets = append(*targets, interactionsTextTarget{
			path:         path + ".content",
			array:        true,
			isTextBlock:  isInteractionsContentTextBlock,
			newTextBlock: newInteractionsContentTextBlock,
		})
		return
	}
	if content.IsObject() {
		if text := content.Get("text"); text.Exists() && text.Type == gjson.String && strings.TrimSpace(text.String()) != "" {
			*targets = append(*targets, interactionsTextTarget{path: path + ".content.text"})
		}
		return
	}
	if text := item.Get("text"); text.Exists() && text.Type == gjson.String && strings.TrimSpace(text.String()) != "" {
		*targets = append(*targets, interactionsTextTarget{path: path + ".text"})
	}
}

func interactionsRole(role, fallback string) string {
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "model", "assistant":
		return "model"
	case "user":
		return "user"
	default:
		return fallback
	}
}

func interactionsArrayHasText(items gjson.Result, isTextBlock func(gjson.Result) bool) bool {
	for _, item := range items.Array() {
		if isTextBlock(item) && strings.TrimSpace(promptBlockText(item)) != "" {
			return true
		}
	}
	return false
}

func isInteractionsNativeTextBlock(block gjson.Result) bool {
	return block.Get("text").Exists()
}

func newInteractionsNativeTextBlock(content string) ([]byte, error) {
	return marshalJSONNoEscape(map[string]any{"text": content})
}

func isInteractionsContentTextBlock(block gjson.Result) bool {
	return block.Type == gjson.String || block.Get("text").Exists()
}

func newInteractionsContentTextBlock(content string) ([]byte, error) {
	return marshalJSONNoEscape(map[string]any{"type": "text", "text": content})
}

func injectInteractionsTarget(payload []byte, target interactionsTextTarget, content, marker, position string) []byte {
	if target.array {
		return blockArrayInject(payload, target.path, target.isTextBlock, target.newTextBlock, content, marker, position)
	}
	current := gjson.GetBytes(payload, target.path)
	if current.Type != gjson.String {
		return payload
	}
	updatedText, mutated := injectIntoText(current.String(), content, marker, position)
	if !mutated {
		return payload
	}
	updated, err := sjson.SetBytes(payload, target.path, updatedText)
	if err != nil {
		return payload
	}
	return updated
}

func stripInteractionsTarget(payload []byte, target interactionsTextTarget, re *regexp.Regexp) []byte {
	if !target.array {
		current := gjson.GetBytes(payload, target.path)
		if current.Type != gjson.String {
			return payload
		}
		stripped := re.ReplaceAllString(current.String(), "")
		if stripped == current.String() {
			return payload
		}
		updated, err := sjson.SetBytes(payload, target.path, stripped)
		if err != nil {
			return payload
		}
		return updated
	}

	items := gjson.GetBytes(payload, target.path)
	if !items.IsArray() {
		return payload
	}
	out := payload
	for i, item := range items.Array() {
		if item.Type == gjson.String {
			stripped := re.ReplaceAllString(item.String(), "")
			if stripped != item.String() {
				if updated, err := sjson.SetBytes(out, fmt.Sprintf("%s.%d", target.path, i), stripped); err == nil {
					out = updated
				}
			}
			continue
		}
		if !target.isTextBlock(item) {
			continue
		}
		text := item.Get("text").String()
		stripped := re.ReplaceAllString(text, "")
		if stripped == text {
			continue
		}
		if updated, err := sjson.SetBytes(out, fmt.Sprintf("%s.%d.text", target.path, i), stripped); err == nil {
			out = updated
		}
	}
	return out
}
