# curlfmt examples

Three self-contained files, all offline.

## messy-api-doc.md

A deliberately rotten API document: a gnarly one-liner, a `$ `-prompted
console block, a pipeline into `jq`, a multi-line JSON body, plus Python
and JSON blocks that must never be touched.

```bash
go build -o curlfmt ./cmd/curlfmt
./curlfmt examples/messy-api-doc.md        # print the rewritten document
./curlfmt lint examples/messy-api-doc.md   # see what is wrong and where
cp examples/messy-api-doc.md /tmp/doc.md && ./curlfmt -w --fix /tmp/doc.md
```

## deploy.sh

A typical deployment script whose curl calls mix short flags, attached
values, `$VAR` expansions, pipelines, and a heredoc. Formatting a copy
shows expansions and pipelines surviving verbatim:

```bash
cp examples/deploy.sh /tmp/deploy.sh && ./curlfmt -w /tmp/deploy.sh
diff examples/deploy.sh /tmp/deploy.sh
```

## ci-check.sh

The CI recipe: `--check` gates formatting, `lint` gates correctness, and
the combined exit status fails the build. Point it at your own docs:

```bash
bash examples/ci-check.sh docs/ README.md; echo "exit: $?"
```

Everything here uses `127.0.0.1` and `example.test`; nothing is ever
fetched — curlfmt reads and rewrites text, it never runs curl.
