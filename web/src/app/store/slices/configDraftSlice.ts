import { createSlice, type PayloadAction } from "@reduxjs/toolkit";

type ConfigDraftState = {
  baseVersion: string | null;
  dirty: boolean;
  applying: boolean;
  conflict: boolean;
};

const initialState: ConfigDraftState = {
  baseVersion: null,
  dirty: false,
  applying: false,
  conflict: false
};

const configDraftSlice = createSlice({
  name: "configDraft",
  initialState,
  reducers: {
    setBaseVersion(state, action: PayloadAction<string | null>) {
      state.baseVersion = action.payload;
    },
    setDirty(state, action: PayloadAction<boolean>) {
      state.dirty = action.payload;
    },
    setApplying(state, action: PayloadAction<boolean>) {
      state.applying = action.payload;
    },
    setConflict(state, action: PayloadAction<boolean>) {
      state.conflict = action.payload;
    }
  }
});

export const { setApplying, setBaseVersion, setConflict, setDirty } =
  configDraftSlice.actions;
export const configDraftReducer = configDraftSlice.reducer;
