import { useCallback, useState } from "react";
import { useTellerConnect, type TellerConnectOnSuccess, type TellerConnectOptions } from "teller-connect-react";

export default function ConnectBank({ sessionToken }: { sessionToken: string }) {
  const [message, setMessage] = useState("");
  const onSuccess = useCallback<TellerConnectOnSuccess>(async (enrollment) => {
    const response = await fetch(`${import.meta.env.VITE_SERVER_BASE_URL ?? ""}/api/teller/connect/complete`, {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ session_token: sessionToken, ...enrollment }),
    });
    setMessage(response.ok ? "Bank connected. You can return to Telegram." : "Unable to connect this account.");
  }, [sessionToken]);
  const config: TellerConnectOptions = { applicationId: import.meta.env.VITE_TELLER_APPLICATION_ID, products: ["transactions"], selectAccount: "single", onSuccess };
  const { open, ready } = useTellerConnect(config);
  return <section><button onClick={() => open()} disabled={!ready || !!message}>Connect bank</button>{message && <p>{message}</p>}</section>;
}
