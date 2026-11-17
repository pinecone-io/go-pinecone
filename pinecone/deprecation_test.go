package pinecone

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Tools such as staticcheck (SA1019) and gopls flag uses of a field only when its doc comment has a
// paragraph that starts with "Deprecated: ".
func TestReadCapacityManualShardsIsMarkedDeprecated(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "models.go", nil, parser.ParseComments)
	require.NoError(t, err)

	var doc string
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok || spec.Name.Name != "ReadCapacityManualScaling" {
			return true
		}
		for _, field := range spec.Type.(*ast.StructType).Fields.List {
			if len(field.Names) == 1 && field.Names[0].Name == "Shards" {
				doc = field.Doc.Text()
			}
		}
		return false
	})

	require.NotEmpty(t, doc, "ReadCapacityManualScaling.Shards has no doc comment")
	require.True(t, strings.Contains(doc, "\n\nDeprecated: "), "doc comment has no Deprecated paragraph:\n%s", doc)
}
