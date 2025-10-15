# ssync

## Introduction

`ssync` stands for "simple sync". It's a CLI tool written in Go. It helps you to make sure that files in two different disks (or directories) are identical and uncorrupted.

## Tutorial

Assume that you have two directories: `source` and `target`.

First, you need to create a manifest file for the `source` directory:

```bash
ssync create ./source ./manifest.csv
```

Hint: You can also use `?` to open a file dialog to select the directory and file. For example:

```bash
ssync create ? ?
```

Then, assume that you modified some files in the `source` directory. You can update the manifest file by running:

```bash
ssync update ./source ./manifest.csv ./manifest_new.csv
```

Notice that you need to provide the original manifest file so that `ssync` can perform a quick incremental update.

Then, assume that you mannually synchronized the `target` directory with the `source` directory. You can compare the directories to check if all files are identical:

```bash
ssync compare ./source ./target
```

By default, `ssync compare` runs in quick mode, which cannot detect file corruption. To enable strict mode, use the `-s` flag:

```bash
ssync compare -s ./source ./target
```

If you find some corrupted files, you can refer to the MD5 hashes in the manifest file to determine the correct copies.
