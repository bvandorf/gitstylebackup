package gitstylebackup

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"
)

// saveRestoreState saves the restore state to a JSON file
func saveRestoreState(stateFile string, state RestoreState) error {
	state.LastUpdate = time.Now().Format(timeFormat)
	
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal restore state: %v", err)
	}
	
	err = ioutil.WriteFile(stateFile, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write restore state file: %v", err)
	}
	
	return nil
}

// loadRestoreState loads the restore state from a JSON file
func loadRestoreState(stateFile string) (RestoreState, error) {
	var state RestoreState
	
	data, err := ioutil.ReadFile(stateFile)
	if err != nil {
		return state, fmt.Errorf("failed to read restore state file: %v", err)
	}
	
	err = json.Unmarshal(data, &state)
	if err != nil {
		return state, fmt.Errorf("failed to unmarshal restore state: %v", err)
	}
	
	return state, nil
}
