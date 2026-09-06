# VS Code for appicon

Open this repository as a folder, or add it as a folder in a multi-root workspace.
Install the recommendations from the Extensions view. Use **Tasks: Run Task** for
the commands below. Tasks run from this repository unless they state another directory.

Use the tool versions documented by the repository. Launch VS Code from the
prepared development shell, or reopen in the existing dev container when available.
Extension recommendations do not install command-line dependencies.

| Task | Command |
| --- | --- |
| make check-fast | `make check-fast` |
| make build | `make build` |
| make test | `make test` |
| make vet | `make vet` |
| make check | `make check` |
| Check diff whitespace | `git diff --check` |

Debug configurations are available in **Run and Debug**. Choose a test or help
configuration for development. The selected-file configurations run the selected
script, so select a test or an intended entry point.
