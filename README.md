# gocraft-abi

The contract between the [GoCraft](https://github.com/GoCraft-MC/GoCraft) server
and every plugin runtime it drives. One schema, and the two pieces of code that
both ends of a connection would otherwise write twice.

It depends on protobuf and, for the manifest, a TOML parser. Nothing else, and
it must stay that way. Everything here
is imported by the server, by the Go SDK and — through generated Java — by the
JVM runtime; a dependency added here is a dependency added to all of them.

## What is in it

| Path | What it is |
| --- | --- |
| `abi/v1/*.proto` | the schema. The single definition of the wire format |
| `abi/v1/wire` | the generated Go types, committed so nothing needs buf to build |
| `abi/v1` | the domain types the rest of the code works on: values, events, commands |
| `ipc` | framing and the wire ↔ domain conversion |
| `command` | the command tree: its shape, its rules, and the two codecs |
| `bundle` | the `.gcpkg` format: the manifest, and reading an archive back |

`command` and `bundle` are formats rather than behaviour. A build tool writes
them and a host reads them, and sharing the code rather than the description is
what lets a build refuse a tree the server would have refused — on the machine
that has the source, rather than on someone's server.

What stays with the host is anything that needs a running world: resolving a
typed line against a permission-pruned view, scanning a drop directory, deciding
where a plugin's data lives.

`abi/v1` and `abi/v1/wire` are two views of the same thing on purpose. The
generated types stop at the `ipc` boundary; the bus, the mutation queue and the
in-process runtimes work on the compact types in `abi/v1`, which cost no
allocation per value. A Lua handler must not pay for a protocol it never speaks.

`ipc` holds only what a host and a plugin both need. Spawning a process,
correlating replies by seq, watching liveness and restarting a dead runtime are
things only a host does, and they live in the server.

## What is not in it

The code generators. `protoc-gen-gocraft` emits the host emitters, the Go SDK
and the Java API from `events.proto`, and it lives in the GoCraft repository
because it knows each host's package layout — `vocabulary.go` maps ABI types to
`core/player`, `core/spatial` and `core/world`. Putting it here would make the
contract depend on the internals of one of its consumers.

## Regenerating

```
buf generate
```

Needs `buf` and `protoc-gen-go` on PATH. The output is committed, so a
contributor who only builds needs neither.

Two versions have to move together: `protoc-gen-go` and the
`google.golang.org/protobuf` in `go.mod`. Gencode from a newer generator calls
methods an older runtime does not have.

## Changing the schema

`buf breaking` is configured with `FILE`, the strictest set — beyond wire
compatibility it rejects source-breaking edits such as renaming an enum value,
which matters as soon as generated Java refers to those names.

A one-sided change here breaks the other side silently. That is the whole reason
this repository exists separately: the schema cannot be edited as a detail of a
server change.
