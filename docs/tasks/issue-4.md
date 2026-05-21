# Tasks for #4: Workflow Engine: DSL Parser & Basic Execution

Parent issue: #4
Parent PRD: docs/PRD_CommerceOS.md

## Tasks

### 1. Define YAML DSL Schema and Go Structs ✅
**Status**: Completed  
**Output**: `internal/workflow/dsl.go`.  
**Implementation**: Created structs mapping to the declarative workflow YAML, including Steps, Dependencies, and Placeholder policies.

### 2. Implement DAG Parser and Validator ✅
**Status**: Completed  
**Output**: `internal/workflow/parser.go`.  
**Implementation**: Built a parser using `gopkg.in/yaml.v3` with built-in topological validation and cycle detection using a DFS algorithm.

### 3. Test DAG Parser and Validator ✅
**Status**: Completed  
**Output**: `internal/workflow/parser_test.go`.  
**Implementation**: Verified valid DAGs, circular dependencies, and duplicate IDs with 100% coverage.

### 4. Implement Basic Workflow Runner ✅
**Status**: Completed  
**Output**: `internal/workflow/runner.go`.  
**Implementation**: Implemented a state-aware execution runner that respects step dependencies and manages the execution flow.

### 5. Integrate Runner with EventBus ✅
**Status**: Completed  
**Implementation**: Connected the runner to `pkg/eventbus`. The runner now emits `workflow.started`, `workflow.step.started`, and `workflow.completed` events.

### 6. Test Workflow Runner and Event Emission ✅
**Status**: Completed  
**Output**: `internal/workflow/runner_test.go`.  
**Implementation**: Verified correct execution order and event metadata emission with 96%+ coverage.
