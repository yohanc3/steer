import { useUserStore } from "../store/useStore";
import LogoutButton from "./LogoutButton";

export default function Dashboard(){
  const { user } = useUserStore()

  return (
    <div>
      <LogoutButton />
      <p>user name: {user.email} </p>
    </div>
  );
}


