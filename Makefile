.RECIPEPREFIX = >

dev:
> find ./ -type f \( -iname \*.templ -o \( -iname \*.go -and -not -iname \*_templ.go \) \) | entr -rz make _build

_build:
> go generate ./...
> go run main.go

.PHONY: dev dev_build
