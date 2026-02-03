import { create } from "zustand";

export type BaseUser = {
  email: string;
  name: string;
  picture: string | null;
  userID: string;
};

interface UserState {
  user: BaseUser;
  setUser: (user: BaseUser) => void;
  clearUser: () => void;
}

export const useUserStore = create<UserState>((set) => ({
  user: { name: "", email: "", picture: "", userID: "" },
  setUser: (user: BaseUser) => set({ user }),
  clearUser: () => set({ user: { name: "", email: "", picture: "", userID: "" } }),
}));

export const useAccessTokenStore = create<{
  accessToken: string;
  setAccessToken: (accessToken: string) => void;
}>((set) => ({
  accessToken: "",
  setAccessToken: (accessToken: string) => set({ accessToken }),
}));
