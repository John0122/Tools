package linklab

import (
	"context"
	"net/http"
	"time"

	"github.com/graphql-go/graphql"
)

// 固定只读 Schema 让扫描样例执行真实查询、变量解析和字段校验。
func newSchema() (graphql.Schema, error) {
	article := graphql.NewObject(graphql.ObjectConfig{Name: "Article", Fields: graphql.Fields{
		"id": &graphql.Field{Type: graphql.ID}, "title": &graphql.Field{Type: graphql.String}, "summary": &graphql.Field{Type: graphql.String},
	}})
	return graphql.NewSchema(graphql.SchemaConfig{Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: graphql.Fields{
		"service": &graphql.Field{Type: graphql.String, Resolve: func(p graphql.ResolveParams) (any, error) { return "target_web", nil }},
		"article": &graphql.Field{Type: article, Args: graphql.FieldConfigArgument{"id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)}}, Resolve: func(p graphql.ResolveParams) (any, error) {
			if p.Args["id"] != "intro" {
				return nil, nil
			}
			return map[string]any{"id": "intro", "title": "外链检测入门", "summary": "对照不同 HTML 特征和真实网络响应，验证链接发现与分类。"}, nil
		}},
		"echo": &graphql.Field{Type: graphql.String, Args: graphql.FieldConfigArgument{"message": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)}}, Resolve: func(p graphql.ResolveParams) (any, error) { return p.Args["message"], nil }},
	}})})
}

func (h *Handler) graphql(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Query         string         `json:"query"`
		Variables     map[string]any `json:"variables"`
		OperationName string         `json:"operationName"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if len(in.Query) == 0 || len(in.Query) > 4096 {
		fail(w, r, 400, "查询不能为空且最多 4096 字节")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	result := graphql.Do(graphql.Params{Schema: h.schema, RequestString: in.Query, VariableValues: in.Variables, OperationName: in.OperationName, Context: ctx})
	code := 200
	if len(result.Errors) > 0 {
		code = 400
	}
	respond(w, r, code, result)
}
