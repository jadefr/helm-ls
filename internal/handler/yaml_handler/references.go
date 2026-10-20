package yamlhandler

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mrjosh/helm-ls/internal/charts"
	"github.com/mrjosh/helm-ls/internal/lsp/symboltable"
	"github.com/mrjosh/helm-ls/internal/util"
	"go.lsp.dev/protocol"
)

// References implements handler.LangHandler.
func (h *YamlHandler) References(ctx context.Context, params *protocol.ReferenceParams) (result []protocol.Location, err error) {
	path, err := h.getYamlPath(params.TextDocument.URI, params.Position)
	if err != nil {
		return nil, fmt.Errorf("Getting References failed for document, could not get YAML Path: %w", err)
	}
	templateContext := symboltable.TemplateContextFromYAMLPath(path)

	logger.Debug("YamlHandler References looking for template context", templateContext)

	definitions, err := h.getDefinitionsInValues(params.TextDocument.URI, templateContext)

	// Scope the template search to the chart tree the document belongs to
	// (its own chart plus any parent/dependency charts), so references from
	// unrelated charts elsewhere in the workspace are not included.
	chart, chartErr := h.chartStore.GetChartOrParentForDoc(params.TextDocument.URI)
	var chartRoot string
	if chartErr == nil {
		chartRoot = chart.RootURI.Filename()
	}
	locations := h.getReferencesInTemplates(templateContext, chartRoot)

	if err != nil {
		return locations, fmt.Errorf("Adding definitions to references failed: %w", err)
	}
	locations = append(locations, definitions...)

	return locations, nil
}

func (h *YamlHandler) getReferencesInTemplates(templateContext symboltable.TemplateContext, chartRoot string) []protocol.Location {
	locations := []protocol.Location{}
	// NOTE: GetAllTemplateDocs requires the chart to be already loaded, currently this happens when
	// getDefinitionsInValues is called
	for _, doc := range h.documents.GetAllTemplateDocs() {
		if chartRoot != "" && !isWithinChart(doc.URI.Filename(), chartRoot) {
			continue
		}

		// TODO(dependecy-charts): template context would need to be adjusted for dependency charts
		// see https://github.com/mrjosh/helm-ls/issues/152
		referenceRanges := doc.SymbolTable.GetTemplateContextRanges(append([]string{"Values"}, templateContext...))

		if len(referenceRanges) > 0 {
			charts.SyncToDisk(doc)
		}

		locations = append(locations, util.RangesToLocations(doc.URI, referenceRanges)...)
	}
	return locations
}

// isWithinChart reports whether path lies inside the chart directory rooted at chartRoot.
func isWithinChart(path, chartRoot string) bool {
	rel, err := filepath.Rel(chartRoot, path)
	if err != nil {
		return false
	}
	return rel == "." || (!strings.HasPrefix(rel, "..") && rel != "")
}
