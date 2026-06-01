SINDRI ?= $(LOCALBIN)/sindri

.PHONY: sindri
sindri: $(SINDRI)
$(SINDRI): $(LOCALBIN)
	@dagger call binary export --path $(SINDRI)

LOCALBIN ?= $(shell pwd)/bin
$(LOCALBIN):
	@mkdir -p $(LOCALBIN)

BIN ?= ~/.local/bin
INSTALL ?= install

.PHONY: install
install: sindri
	@$(INSTALL) $(SINDRI) $(BIN)
