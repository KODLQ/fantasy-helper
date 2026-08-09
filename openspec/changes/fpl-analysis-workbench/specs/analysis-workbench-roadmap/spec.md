## ADDED Requirements

### Requirement: Coordinate authoritative analysis delivery changes

The workbench SHALL treat `fpl-league-gameweek-insights`, `fpl-transfer-fixture-research`, `fpl-recommendation-evaluation`, and `fpl-live-alerts-storyline` as the authoritative owners of their domain requirements, APIs, formulas, persistence, UI, and Playwright tests. The umbrella SHALL own only shared integration gates.

#### Scenario: Child change is implemented
- **WHEN** a child analysis change is proposed or implemented
- **THEN** its domain specification is validated independently and the umbrella verifies only dependency, common-contract, ownership, navigation, handoff, and cross-workbench browser compatibility

#### Scenario: Duplicate domain requirement is added to the umbrella
- **WHEN** a parent artifact attempts to define a child-owned domain API, formula, persistence rule, or UI acceptance behavior
- **THEN** the requirement is rejected or moved to the appropriate child change instead of becoming a competing specification

### Requirement: Use one shared formula registry

The workbench SHALL maintain `openspec/formulas.yaml` as the canonical registry for shared derived metrics, normalization behavior, alert thresholds, and ranking formulas. Child changes SHALL reference a formula ID/version and SHALL NOT redefine a conflicting formula.

#### Scenario: Child consumes a shared formula
- **WHEN** a child change calculates fixture ease, team points, point differences, recommendation metrics, alert thresholds, or storyline impact
- **THEN** it references the canonical formula ID/version and returns that version in provenance

#### Scenario: Shared formula changes
- **WHEN** a formula definition or normalization rule changes
- **THEN** the registry version is updated, dependent child tests are rerun, and prior analysis results remain tied to their original formula version
