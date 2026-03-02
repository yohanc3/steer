import { useUserStore } from "../store/useStore";
import ConnectToTelegramBtn from "./ConnectToTelegramBtn";
import LogoutButton from "./LogoutButton";

export default function Dashboard(){
  const { user } = useUserStore()



  return (
    <div> 
        <LogoutButton />
        <p>user name: {user.email} </p>
        
        <ConnectToTelegramBtn/>
    </div>
  );
}


