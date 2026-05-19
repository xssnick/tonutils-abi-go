package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: "15:04:05",
	}).With().Timestamp().Logger()

	var (
		abiPath = flag.String("abi", "", "path to Tolk ABI json")
		outPath = flag.String("out", "", "path to generated Go file, stdout when empty")
		pkgName = flag.String("package", "wrappers", "generated Go package name")
		strict  = flag.Bool("strict", false, "fail when unsupported ABI constructs would be emitted as TODO comments or schema diagnostics")
	)
	flag.Parse()

	if *abiPath == "" && flag.NArg() > 0 {
		*abiPath = flag.Arg(0)
	}
	if *abiPath == "" {
		logger.Fatal().Msg("missing ABI path, pass -abi <file> or positional file")
	}

	result, err := GenerateFile(*abiPath, Options{
		PackageName: *pkgName,
		Strict:      *strict,
	})
	if err != nil {
		logger.Fatal().Err(err).Msg("generate")
	}
	for _, diagnostic := range result.Diagnostics {
		event := logger.Warn().Str("code", string(diagnostic.Code))
		if diagnostic.Subject != "" {
			event = event.Str("subject", diagnostic.Subject)
		}
		message := diagnostic.Message
		if message == "" {
			message = diagnostic.String()
		}
		event.Msg(message)
	}
	for _, custom := range result.CustomSerializers {
		var setters []string
		if custom.LoadFromCellSetterName != "" {
			setters = append(setters, custom.LoadFromCellSetterName)
		}
		if custom.ToCellSetterName != "" {
			setters = append(setters, custom.ToCellSetterName)
		}
		if len(setters) == 0 {
			continue
		}
		logger.Warn().
			Str("type", custom.TypeName).
			Str("functions", strings.Join(setters, ", ")).
			Msg("custom pack/unpack must be assigned manually")
	}

	if *outPath == "" {
		_, _ = os.Stdout.Write(result.Source)
		return
	}
	if err := os.WriteFile(*outPath, result.Source, 0o644); err != nil {
		logger.Fatal().Err(err).Msg("write generated file")
	}
	absPath, err := filepath.Abs(*outPath)
	if err != nil {
		logger.Info().Str("file", *outPath).Msg("generated file")
		return
	}
	logger.Info().Str("file", absPath).Msg("generated file")
}
