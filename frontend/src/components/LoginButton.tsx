import { useAuth0 } from "@auth0/auth0-react";
import { Button } from "./ui/button";

export default function LoginButton() {
    const { loginWithRedirect } = useAuth0();

    return (
        <Button onClick={() => loginWithRedirect()} variant="default" className="cursor-pointer">
            Log In
        </Button>
    );
}
