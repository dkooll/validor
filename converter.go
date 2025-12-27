package validor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclwrite"
	"github.com/zclconf/go-cty/cty"
)

type DefaultSourceConverter struct {
	registryClient RegistryClient
}

func NewSourceConverter(client RegistryClient) SourceConverter {
	return &DefaultSourceConverter{
		registryClient: client,
	}
}

func (c *DefaultSourceConverter) ConvertToLocal(ctx context.Context, modulePath string, moduleInfo ModuleInfo) ([]FileRestore, error) {
	var filesToRestore []FileRestore

	files, err := filepath.Glob(filepath.Join(modulePath, "*.tf"))
	if err != nil {
		return nil, fmt.Errorf("failed to find terraform files: %w", err)
	}

	moduleSource := fmt.Sprintf("%s/%s/%s", moduleInfo.Namespace, moduleInfo.Name, moduleInfo.Provider)
	submodulePattern := fmt.Sprintf(`^%s/%s/%s//modules/(.*)$`,
		regexp.QuoteMeta(moduleInfo.Namespace),
		regexp.QuoteMeta(moduleInfo.Name),
		regexp.QuoteMeta(moduleInfo.Provider))
	submoduleRegex := regexp.MustCompile(submodulePattern)

	for _, file := range files {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		content, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		originalContent := string(content)
		parsedFile, diags := hclwrite.ParseConfig(content, file, hcl.InitialPos)
		if diags.HasErrors() {
			return filesToRestore, fmt.Errorf("failed to parse %s: %s", file, diags.Error())
		}

		if !c.updateModuleBlocks(parsedFile.Body(), moduleSource, submoduleRegex) {
			continue
		}

		if err := os.WriteFile(file, parsedFile.Bytes(), 0644); err != nil {
			return filesToRestore, fmt.Errorf("failed to write file %s: %w", file, err)
		}

		filesToRestore = append(filesToRestore, FileRestore{
			Path:            file,
			OriginalContent: originalContent,
			ModuleName:      moduleInfo.Name,
			Provider:        moduleInfo.Provider,
			Namespace:       moduleInfo.Namespace,
		})
	}

	return filesToRestore, nil
}

func (c *DefaultSourceConverter) RevertToRegistry(ctx context.Context, filesToRestore []FileRestore) error {
	for _, restore := range filesToRestore {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		latestVersion, err := c.registryClient.GetLatestVersion(ctx, restore.Namespace, restore.ModuleName, restore.Provider)
		if err != nil {
			if writeErr := os.WriteFile(restore.Path, []byte(restore.OriginalContent), 0644); writeErr != nil {
				return fmt.Errorf("failed to restore file %s: %w", restore.Path, writeErr)
			}
			continue
		}

		updatedContent := c.updateVersionInContent(restore.OriginalContent, latestVersion)

		if err := os.WriteFile(restore.Path, []byte(updatedContent), 0644); err != nil {
			return fmt.Errorf("failed to write updated file %s: %w", restore.Path, err)
		}
	}
	return nil
}

func (c *DefaultSourceConverter) updateVersionInContent(content, latestVersion string) string {
	versionRegex := regexp.MustCompile(`(version\s*=\s*")[^"]*(")`)
	if versionRegex.MatchString(content) {
		return versionRegex.ReplaceAllString(content, fmt.Sprintf("${1}~> %s${2}", latestVersion))
	}
	return content
}

func (c *DefaultSourceConverter) updateModuleBlocks(body *hclwrite.Body, moduleSource string, submoduleRegex *regexp.Regexp) bool {
	changed := false
	for _, block := range body.Blocks() {
		if block.Type() == "module" && c.updateModuleBlock(block, moduleSource, submoduleRegex) {
			changed = true
		}
		if c.updateModuleBlocks(block.Body(), moduleSource, submoduleRegex) {
			changed = true
		}
	}
	return changed
}

func (c *DefaultSourceConverter) updateModuleBlock(block *hclwrite.Block, moduleSource string, submoduleRegex *regexp.Regexp) bool {
	attr := block.Body().GetAttribute("source")
	if attr == nil {
		return false
	}

	sourceValue, ok := attributeStringValue(attr)
	if !ok {
		return false
	}

	switch {
	case sourceValue == moduleSource:
		block.Body().SetAttributeValue("source", cty.StringVal("../../"))
		block.Body().RemoveAttribute("version")
		return true
	case submoduleRegex != nil:
		if matches := submoduleRegex.FindStringSubmatch(sourceValue); len(matches) == 2 {
			localPath := fmt.Sprintf("../../modules/%s", strings.TrimPrefix(matches[1], "/"))
			block.Body().SetAttributeValue("source", cty.StringVal(localPath))
			block.Body().RemoveAttribute("version")
			return true
		}
	}

	return false
}

func attributeStringValue(attr *hclwrite.Attribute) (string, bool) {
	tokens := attr.Expr().BuildTokens(nil)
	if len(tokens) == 0 {
		return "", false
	}
	raw := strings.TrimSpace(string(tokens.Bytes()))
	if raw == "" {
		return "", false
	}
	value, err := strconv.Unquote(raw)
	if err != nil {
		return "", false
	}
	return value, true
}
