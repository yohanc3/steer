import { useUserStore, type BaseUser } from "../store/userStore";
import LogoutButton from "./LogoutButton";
import useUser from "../model/User";
import { useEffect } from "react";

export default function Dashboard({ user }: { user: BaseUser }) {
  const { setUser } = useUserStore();
  const { onUserLogin } = useUser(user);

  // Push user to db in the backend only if not logged in
  useEffect(() => {
    // Using "logged_in" is kinda janky but it's enough for a simple check
    console.log("based user: ", user)
    if (localStorage.getItem("authStatus") === "loggedIn") return;
    onUserLogin();

    setUser({ ...user });

    localStorage.setItem("authStatus", "loggedIn");
  }, [user, onUserLogin, setUser]);


  return (
    <div>
      <LogoutButton />
      <p>user name: {user.userID}</p>
    </div>
  );
}
