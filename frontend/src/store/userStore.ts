import { create } from "zustand";

type BaseUser = {
  email: string;
  name: string | undefined;
  picture: string | undefined;
};

interface UserState {
  user: BaseUser;
  setUser: (user: BaseUser) => void;
  clearUser: () => void;
}

export const useUserStore = create<UserState>((set) => ({
  user: { name: "", email: "", picture: "" },
  setUser: (user: BaseUser) => set({ user }),
  clearUser: () => set({ user: { name: "", email: "", picture: "" } }),
  
}));
