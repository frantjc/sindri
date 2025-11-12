.PHONY: install
install:
	@mkdir -p ~/.local/bin
	@dagger call binary export --path ~/.local/bin/sindri

.PHONY: .git/hooks .git/hooks/
.git/hooks .git/hooks/:
	@cp .githooks/* $@

.PHONY: .git/hooks/pre-commit
.git/hooks/pre-commit:
	@cp .githooks/pre-commit $@
