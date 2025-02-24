# tag

tag is a tool meant for CI pipelines that automates and simplifies creating git tags for your repo.

## How It Works

This repo dog-foods itself and uses this tool to generate git tags. An example of how to use this can be found in the `tag-and-release.yml` Github workflow.

These are the general steps it takes:

1. Read configuration information from the environment, a config file (optional), a `VERSION` file (optional), and the CLI parameters.
2. Determine what the requested git tag version is.
3. Determine what the latest git tag version is, using traditional semver ordering.
4. If the requested version is greater than the largest existing version, tag it.

After tagging is complete, you must still do a `git push --tags --force` to update the remote repo.

## Example

```
$ go run github.com/chigopher/tag@latest tag
5:31PM INF found largest semver tag previous-version=0.0.0
5:31PM INF failed to delete tag, but probably not an issue. error="tag not found" tag=v0.1.0
5:31PM INF failed to delete tag, but probably not an issue. error="tag not found" tag=v0
5:31PM INF tag successfully created tag=v0.1.0
5:31PM INF created new tag. Push to origin still required. previous-version=0.0.0 requested-version=0.1.0
versions: v0.1.0,v0.0.0
```

## Exit Codes

The tool exits with special exit codes that have specific meanings:

| Exit Code | Meaning |
|-----------|---------|
| `0` | A tag was successfully created. |
| `8` | No new tags were created. |
| (anything else) | An unknown error occurred. |

## Stdout

The tool prints information to stdout that can be used to parse the results of the operation. For example:

```
$ go run github.com/chigopher/tag@latest tag 2>/dev/null
v0.1.1,v0.0.0
```

The output is a CSV with the following meanings assigned to the field indexes:

| Index | Meaning |
|-------|---------|
| `0` | The new git tag that was created. |
| `1` | The previously largest git tag that existed in the repo. |

## Configuration

All the config options can be specified from multiple locations:

1. Environment variables prefixed with `TAG_`.
2. A `.tag.yml` config file.

Additionally, a special `VERSION` file can be specified that is a single-line file that contains the requested git tag. This file is optional, as the same information can be provided in the `version` config field (either in the config file or as an environment variable).

Both the `.tag.yml` file and the `VERSION` file can be discovered automatically by `tag` by searching every path from the current working directory up to the root directory.