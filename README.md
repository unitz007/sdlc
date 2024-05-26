# SDLC
SDLC is an abstraction over the common software development processes, viz: run, test and build

## Installation
Build from source using [git](https://git-scm.com) and [go](https://go.dev)

```bash
git clone https://github.com/unitz007/sdlc
cd sdlc
go build -v
````

## Usage
```bash
./sdlc --help
```
## Configure builds
```json
{
  "pom.xml": {
    "run": "mvn spring-boot:run",
    "test": "mvn test",
    "build": "mvn build"
  },
  "go.mod": {
    "run": "go run main.go",
    "test": "go test .",
    "build": "go build -v"
  }
}
```

## Contributing
Pull requests are welcome. For major changes, please open an issue first
to discuss what you would like to change.

Please make sure to update tests as appropriate.

## License
[Apache](http://www.apache.org/licenses/LICENSE-2.0)