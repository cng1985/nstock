package web

import "embed"

// Dist 是 Vite 构建出的前端，编译时打进二进制。
//
//go:embed all:dist
var Dist embed.FS
