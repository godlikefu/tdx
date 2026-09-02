package httpserver

import (
	_ "embed"
	"net/http"
)

// 接口文档端点: /openapi.yaml 下载规范, /docs 在线预览(Swagger UI)。
// 规范文件由路由注册机械提取生成并经路径集校验, 修改路由后需同步重新生成。

//go:embed openapi.yaml
var openapiSpec []byte

// handleOpenAPI 提供 OpenAPI 规范下载
func handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="openapi.yaml"`)
	_, _ = w.Write(openapiSpec)
}

// handleDocs Swagger UI 页面(CDN 版, 浏览器需可访问 unpkg.com;
// 离线场景直接下载 /openapi.yaml 导入 Apifox/Postman)
func handleDocs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(docsHTML))
}

const docsHTML = `<!DOCTYPE html>
<html lang="zh">
<head>
<meta charset="utf-8"/>
<meta name="viewport" content="width=device-width, initial-scale=1"/>
<title>tdx HTTP API 文档</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css"/>
</head>
<body>
<div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>
window.addEventListener('load', function () {
  window.ui = SwaggerUIBundle({
    url: '/openapi.yaml',
    dom_id: '#swagger-ui',
    deepLinking: true,
    persistAuthorization: false
  });
});
</script>
</body>
</html>
`
