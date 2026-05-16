import type { User } from "@/types/types";
import { create } from "zustand";

interface UserState {
    user: User;
    setUser: (user: User) => void;
    clearUser: () => void;
}

const initialUser = { name: "", email: "", picture: "", ID: "", isConnectedToTelegram: false };

export const useUserStore = create<UserState>((set) => ({
    user: initialUser,
    setUser: (user: User) => set({ user }),
    clearUser: () => set({ user: initialUser }),
}));
