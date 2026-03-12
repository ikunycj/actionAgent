import { createSlice, type PayloadAction } from "@reduxjs/toolkit";

type ChatUiState = {
  currentInput: string;
  activeSessionKey: string | null;
  streamingMessage: string;
};

const initialState: ChatUiState = {
  currentInput: "",
  activeSessionKey: null,
  streamingMessage: ""
};

const chatUiSlice = createSlice({
  name: "chatUi",
  initialState,
  reducers: {
    setCurrentInput(state, action: PayloadAction<string>) {
      state.currentInput = action.payload;
    },
    setActiveSessionKey(state, action: PayloadAction<string | null>) {
      state.activeSessionKey = action.payload;
    },
    setStreamingMessage(state, action: PayloadAction<string>) {
      state.streamingMessage = action.payload;
    }
  }
});

export const { setActiveSessionKey, setCurrentInput, setStreamingMessage } =
  chatUiSlice.actions;
export const chatUiReducer = chatUiSlice.reducer;
