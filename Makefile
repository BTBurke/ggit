.RECIPEPREFIX = >

dev:
> find ./ -type f \( -iname \*.templ -o \( -iname \*.go -and -not -iname \*_templ.go \) \) | entr -rz make _build

_build:
> go generate ./...
> go run main.go

deps:
> go install github.com/a-h/templ/cmd/templ@latest
> go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest

.PHONY: dev _build
