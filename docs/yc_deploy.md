## yc deploy

Deploy your serverless, this is an alias of chaining commands (upload -> remove -> create)

```
yc deploy src_file[.go|.zip|dir] [flags]
```

### Options

```
      --env stringArray   Set environment variables
  -h, --help              help for deploy
```

### Options inherited from parent commands

```
      --secret string   Vivgrid App secret
      --tool string     Serverless LLM Tool name (default "my_first_llm_tool")
      --zipper string   Vivgrid zipper endpoint (default "zipper.vivgrid.com:9000")
```

### SEE ALSO

* [yc](yc.md)	 - Manage your globally deployed Serverless LLM Functions on vivgrid.com from the command line

