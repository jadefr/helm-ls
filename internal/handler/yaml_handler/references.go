package yamlhandler

import (
	"context"
	"fmt"

	"github.com/mrjosh/helm-ls/internal/charts"
	"github.com/mrjosh/helm-ls/internal/lsp/symboltable"
	"github.com/mrjosh/helm-ls/internal/util"
	"go.lsp.dev/protocol"
)

// References implements handler.LangHandler.
func (h *YamlHandler) References(ctx context.Context, params *protocol.ReferenceParams) (result []protocol.Location, err error) {
	// Store the current document's URI
	h.currentURI = params.TextDocument.URI

	path, err := h.getYamlPath(params.TextDocument.URI, params.Position)
	if err != nil {
		return nil, fmt.Errorf("Getting References failed for document, could not get YAML Path: %w", err)
	}
	templateContext := symboltable.TemplateContextFromYAMLPath(path)

	logger.Debug("YamlHandler References looking for template context", templateContext)

	    // Always get references in templates first
    locations := h.getReferencesInTemplates(templateContext)
 
    // Try to get definitions in values, but don't fail if it errors
    definitions, err := h.getDefinitionsInValues(params.TextDocument.URI, templateContext)
    if err != nil {
        logger.Debug("Could not get definitions in values, continuing with template references only:", err)
    } else {
        locations = append(locations, definitions...)
    }

	return locations, nil
}

func (h *YamlHandler) getReferencesInTemplates(templateContext symboltable.TemplateContext) []protocol.Location {
	locations := []protocol.Location{}
	// NOTE: GetAllTemplateDocs requires the chart to be already loaded, currently this happens when
	// getDefinitionsInValues is called

	// Get the chart for the current values.yaml file
	currentChart, err := h.chartStore.GetChartForDoc(h.currentURI)
	if err != nil {
		logger.Error("Error getting chart for document:", err)
		return locations
	}

	for _, doc := range h.documents.GetAllTemplateDocs() {
		// Skip if template is not from the current chart
		docChart, err := h.chartStore.GetChartForDoc(doc.URI)
		if err != nil || docChart != currentChart {
			continue
		}

		referenceRanges := doc.SymbolTable.GetTemplateContextRanges(append([]string{"Values"}, templateContext...))

		if len(referenceRanges) > 0 {
			charts.SyncToDisk(doc)
		}

		locations = append(locations, util.RangesToLocations(doc.URI, referenceRanges)...)
	}
	return locations
}
