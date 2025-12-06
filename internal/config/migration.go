package config

import (
	"fmt"
	"sort"
)

// Version represents a configuration version.
type Version struct {
	Major int
	Minor int
	Patch int
}

// String returns the version as a string.
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Compare compares two versions.
// Returns -1 if v < other, 0 if v == other, 1 if v > other.
func (v Version) Compare(other Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}
	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}
	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}
	return 0
}

// Migration represents a configuration migration from one version to another.
type Migration struct {
	// FromVersion is the source version.
	FromVersion Version

	// ToVersion is the target version.
	ToVersion Version

	// Description describes what the migration does.
	Description string

	// Migrate performs the migration on the configuration data.
	// It returns the migrated data and any error encountered.
	Migrate func(data map[string]any) (map[string]any, error)
}

// Migrator handles configuration migrations between versions.
type Migrator struct {
	migrations []Migration
	current    Version
}

// NewMigrator creates a new Migrator with the current version.
func NewMigrator(current Version) *Migrator {
	return &Migrator{
		current:    current,
		migrations: make([]Migration, 0),
	}
}

// CurrentVersion returns the current configuration version.
func (m *Migrator) CurrentVersion() Version {
	return m.current
}

// Register adds a migration to the migrator.
func (m *Migrator) Register(migration Migration) {
	m.migrations = append(m.migrations, migration)
	// Sort migrations by (FromVersion, ToVersion) for deterministic ordering
	sort.Slice(m.migrations, func(i, j int) bool {
		cmp := m.migrations[i].FromVersion.Compare(m.migrations[j].FromVersion)
		if cmp != 0 {
			return cmp < 0
		}
		return m.migrations[i].ToVersion.Compare(m.migrations[j].ToVersion) < 0
	})
}

// NeedsMigration checks if the configuration needs migration.
func (m *Migrator) NeedsMigration(data map[string]any) bool {
	v := m.extractVersion(data)
	return v.Compare(m.current) < 0
}

// ErrNewerVersion is returned when the configuration version is newer than the migrator's current version.
var ErrNewerVersion = fmt.Errorf("configuration version is newer than current version")

// ErrMigrationGap is returned when no migration exists for the current data version.
var ErrMigrationGap = fmt.Errorf("no migration registered for current data version")

// Migrate performs all necessary migrations to bring the configuration
// to the current version. Migrations are applied sequentially, requiring
// each migration's FromVersion to match the current data version.
//
// Returns ErrNewerVersion if the data version is newer than the migrator's current version.
// Returns ErrMigrationGap if no migration exists for the current data version.
func (m *Migrator) Migrate(data map[string]any) (map[string]any, []MigrationResult, error) {
	dataVersion := m.extractVersion(data)
	results := make([]MigrationResult, 0)

	// Check if data version is newer than current - don't downgrade
	if dataVersion.Compare(m.current) > 0 {
		return data, results, fmt.Errorf("%w: data version %s > current %s",
			ErrNewerVersion, dataVersion, m.current)
	}

	// Already at current version - nothing to do
	if dataVersion.Compare(m.current) == 0 {
		return data, results, nil
	}

	// Apply migrations sequentially, matching FromVersion exactly
	for dataVersion.Compare(m.current) < 0 {
		// Find a migration that starts from the current data version
		var foundMigration *Migration
		for i := range m.migrations {
			migration := &m.migrations[i]
			if migration.FromVersion.Compare(dataVersion) == 0 &&
				migration.ToVersion.Compare(m.current) <= 0 {
				foundMigration = migration
				break
			}
		}

		if foundMigration == nil {
			return data, results, fmt.Errorf("%w: no migration from version %s",
				ErrMigrationGap, dataVersion)
		}

		// Apply the migration
		migrated, err := foundMigration.Migrate(data)
		result := MigrationResult{
			FromVersion: foundMigration.FromVersion,
			ToVersion:   foundMigration.ToVersion,
			Description: foundMigration.Description,
		}

		if err != nil {
			result.Error = err
			results = append(results, result)
			return data, results, fmt.Errorf("migration from %s to %s failed: %w",
				foundMigration.FromVersion, foundMigration.ToVersion, err)
		}

		result.Success = true
		results = append(results, result)
		data = migrated
		dataVersion = foundMigration.ToVersion
	}

	// Update the version to current after successful migration
	data["_version"] = m.current.String()

	return data, results, nil
}

// MigrationResult contains the result of a single migration.
type MigrationResult struct {
	FromVersion Version
	ToVersion   Version
	Description string
	Success     bool
	Error       error
}

// extractVersion extracts the version from configuration data.
func (m *Migrator) extractVersion(data map[string]any) Version {
	vStr, ok := data["_version"].(string)
	if !ok {
		// No version means initial version 0.0.0
		return Version{}
	}

	var v Version
	_, _ = fmt.Sscanf(vStr, "%d.%d.%d", &v.Major, &v.Minor, &v.Patch)
	return v
}

// DefaultMigrator returns a migrator with default migrations registered.
func DefaultMigrator() *Migrator {
	m := NewMigrator(Version{Major: 1, Minor: 0, Patch: 0})

	// Register migrations here as the configuration format evolves.
	// Example migration:
	// m.Register(Migration{
	//     FromVersion: Version{0, 0, 0},
	//     ToVersion:   Version{1, 0, 0},
	//     Description: "Initial migration to v1.0.0",
	//     Migrate: func(data map[string]any) (map[string]any, error) {
	//         // Perform migration
	//         return data, nil
	//     },
	// })

	return m
}

// MigrationRename creates a migration that renames a configuration path.
func MigrationRename(from, to Version, oldPath, newPath, description string) Migration {
	return Migration{
		FromVersion: from,
		ToVersion:   to,
		Description: description,
		Migrate: func(data map[string]any) (map[string]any, error) {
			value, found := getNestedValue(data, oldPath)
			if !found {
				return data, nil // Nothing to migrate
			}

			// Set new path
			if err := setNestedValue(data, newPath, value); err != nil {
				return nil, fmt.Errorf("setting %s: %w", newPath, err)
			}

			// Remove old path
			deleteNestedValue(data, oldPath)

			return data, nil
		},
	}
}

// MigrationTransform creates a migration that transforms a value at a path.
func MigrationTransform(from, to Version, path, description string, transform func(any) (any, error)) Migration {
	return Migration{
		FromVersion: from,
		ToVersion:   to,
		Description: description,
		Migrate: func(data map[string]any) (map[string]any, error) {
			value, found := getNestedValue(data, path)
			if !found {
				return data, nil // Nothing to transform
			}

			newValue, err := transform(value)
			if err != nil {
				return nil, fmt.Errorf("transforming %s: %w", path, err)
			}

			if err := setNestedValue(data, path, newValue); err != nil {
				return nil, fmt.Errorf("setting %s: %w", path, err)
			}

			return data, nil
		},
	}
}

// MigrationDelete creates a migration that deletes a configuration path.
func MigrationDelete(from, to Version, path, description string) Migration {
	return Migration{
		FromVersion: from,
		ToVersion:   to,
		Description: description,
		Migrate: func(data map[string]any) (map[string]any, error) {
			deleteNestedValue(data, path)
			return data, nil
		},
	}
}

// getNestedValue retrieves a value from a nested map using a dot-separated path.
func getNestedValue(data map[string]any, path string) (any, bool) {
	parts := splitPath(path)
	if len(parts) == 0 {
		return nil, false
	}

	current := any(data)
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}

	return current, true
}

// setNestedValue sets a value in a nested map using a dot-separated path.
func setNestedValue(data map[string]any, path string, value any) error {
	parts := splitPath(path)
	if len(parts) == 0 {
		return ErrInvalidPath
	}

	current := data
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		next, ok := current[part]
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		nextMap, ok := next.(map[string]any)
		if !ok {
			return ErrInvalidPath
		}
		current = nextMap
	}

	current[parts[len(parts)-1]] = value
	return nil
}

// deleteNestedValue deletes a value from a nested map using a dot-separated path.
func deleteNestedValue(data map[string]any, path string) {
	parts := splitPath(path)
	if len(parts) == 0 {
		return
	}

	current := data
	for i := 0; i < len(parts)-1; i++ {
		part := parts[i]
		next, ok := current[part]
		if !ok {
			return
		}
		nextMap, ok := next.(map[string]any)
		if !ok {
			return
		}
		current = nextMap
	}

	delete(current, parts[len(parts)-1])
}
