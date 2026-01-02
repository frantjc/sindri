SINDRI ?= $(LOCALBIN)/sindri

.PHONY: sindri
sindri: $(SINDRI)
$(SINDRI): $(LOCALBIN)
	@dagger call binary export --path $(SINDRI)

.PHONY: .git/hooks .git/hooks/ .git/hooks/pre-commit
.git/hooks .git/hooks/ .git/hooks/pre-commit:
	@cp .githooks/* .git/hooks

.PHONY: release
release:
	@git tag $(SEMVER)
	@git push
	@git push --tags

LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	@mkdir -p $(LOCALBIN)

BIN ?= ~/.local/bin
INSTALL ?= install

.PHONY: install
install: sindri
	@$(INSTALL) $(SINDRI) $(BIN)
