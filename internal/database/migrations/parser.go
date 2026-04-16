package migrations

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
)

var ErrInvalidVersion = errors.New("invalid version")

const (
	delimUp   = `-- up`
	delimDown = `-- down`

	migrationExt = `.sql`

	stateStart = 0
	stateUp    = 1
	stateDown  = 2
)

func parse(f fs.File) (*Migration, error) {
	scanner := bufio.NewScanner(f)

	info, err := f.Stat()
	if err != nil {
		return nil, fmt.Errorf("f.Stat: %w", err)
	}

	version, name, err := parseFileName(info.Name())
	if err != nil {
		return nil, fmt.Errorf("parseFileName: %w", err)
	}

	migration := Migration{
		Version: version,
		Name:    name,
	}

	currentParse := stateStart
	for scanner.Scan() {
		str := scanner.Text()
		switch str {
		case delimUp:
			currentParse = stateUp

			continue
		case delimDown:
			currentParse = stateDown

			continue
		}

		switch currentParse {
		case stateUp:
			migration.Up += str
		case stateDown:
			migration.Down += str
		}
	}

	err = scanner.Err()
	if err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	return &migration, nil
}

func parseFileName(fName string) (uint, string, error) {
	ext := filepath.Ext(fName)
	if ext != migrationExt {
		return 0, "", ErrInvalidMigrationExt
	}

	const partsCount = 3
	slice := strings.Split(fName, ".")
	if len(slice) != partsCount {
		return 0, "", ErrInvalidMigrationName
	}

	version, err := strconv.Atoi(slice[0])
	if err != nil {
		return 0, "", fmt.Errorf("strconv.Atoi: %w", err)
	}

	if version < 0 {
		return 0, "", fmt.Errorf("%w: %d", ErrInvalidVersion, version)
	}

	return uint(version), slice[1], nil
}
