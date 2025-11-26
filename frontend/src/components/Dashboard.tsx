import { useEffect } from "react";
import useUser from "../model/User";
import { useUserStore } from "../store/userStore";
import LogoutButton from "./LogoutButton";

export default function Dashboard() {
  const user = useUserStore((state) => state.user);
  const { onUserLogin } = useUser(user.email);

  // Push user to db in the backend only if not logged in
  useEffect(() => {
    // Using "logged_in" is kinda janky but it's enough for a simple check
    if (localStorage.getItem("authStatus") === "loggedIn") return;

    onUserLogin();

    localStorage.setItem("authStatus", "loggedIn");
  }, [user.email, onUserLogin]);

  return (
    <div>
      <LogoutButton />
      <p>user name: {user.name}</p>
    </div>
  );
}
