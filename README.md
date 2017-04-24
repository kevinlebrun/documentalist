# Documentalist

I use this little bot to generate and serve the documentation of my projects.

The bot need a configuration file at the project root named `.documentalist.json` with the following content:

```json
{
    "command": ["make", "doc"],
    "path": "build/doc",
    "notify": true
}
```

## Usage

```
$ go build .
$ ./documentalist -h
```

## License

(The MIT License)
