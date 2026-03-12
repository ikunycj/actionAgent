import { createSlice, type PayloadAction } from "@reduxjs/toolkit";

type LayoutState = {
  sidebarOpen: boolean;
  theme: "light";
  drawerTaskId: string | null;
};

const initialState: LayoutState = {
  sidebarOpen: true,
  theme: "light",
  drawerTaskId: null
};

const layoutSlice = createSlice({
  name: "layout",
  initialState,
  reducers: {
    toggleSidebar(state) {
      state.sidebarOpen = !state.sidebarOpen;
    },
    setSidebarOpen(state, action: PayloadAction<boolean>) {
      state.sidebarOpen = action.payload;
    },
    setDrawerTaskId(state, action: PayloadAction<string | null>) {
      state.drawerTaskId = action.payload;
    }
  }
});

export const { setDrawerTaskId, setSidebarOpen, toggleSidebar } =
  layoutSlice.actions;
export const layoutReducer = layoutSlice.reducer;
