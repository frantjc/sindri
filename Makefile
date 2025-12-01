.PHONY: install
install:
	@mkdir -p ~/.local/bin
	@dagger call binary export --path ~/.local/bin/sindri

.PHONY: .git/hooks .git/hooks/ .git/hooks/pre-commit
.git/hooks .git/hooks/ .git/hooks/pre-commit:
	@cp .githooks/* .git/hooks
