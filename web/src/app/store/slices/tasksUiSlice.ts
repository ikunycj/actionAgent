import { createSlice, type PayloadAction } from "@reduxjs/toolkit";

type TasksUiState = {
  activeTaskId: string | null;
  listLimit: number;
  autoRefresh: boolean;
};

const initialState: TasksUiState = {
  activeTaskId: null,
  listLimit: 20,
  autoRefresh: true
};

const tasksUiSlice = createSlice({
  name: "tasksUi",
  initialState,
  reducers: {
    setActiveTaskId(state, action: PayloadAction<string | null>) {
      state.activeTaskId = action.payload;
    },
    setListLimit(state, action: PayloadAction<number>) {
      state.listLimit = action.payload;
    },
    setAutoRefresh(state, action: PayloadAction<boolean>) {
      state.autoRefresh = action.payload;
    }
  }
});

export const { setActiveTaskId, setAutoRefresh, setListLimit } =
  tasksUiSlice.actions;
export const tasksUiReducer = tasksUiSlice.reducer;
