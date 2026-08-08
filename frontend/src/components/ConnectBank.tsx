import { useCallback, useState } from "react";
import { useTellerConnect, type TellerConnectOnSuccess, type TellerConnectOptions } from "teller-connect-react";

export default function ConnectBank({ sessionToken, nonce }: { sessionToken: string; nonce: string }) {
  const [message, setMessage] = useState("");
  const onSuccess = useCallback<TellerConnectOnSuccess>(async (enrollment) => {
    const response = await fetch(`${import.meta.env.VITE_SERVER_BASE_URL ?? ""}/api/teller/connect/complete`, {
      method: "POST", headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        session_token: sessionToken,
        access_token: enrollment.accessToken,
        enrollment: enrollment.enrollment,
        user: enrollment.user,
        signatures: enrollment.signatures ?? [],
      }),
    });
    setMessage(response.ok ? "Bank connected. You can return to Telegram." : "Unable to connect this account.");
  }, [sessionToken]);
  const environment = import.meta.env.VITE_TELLER_ENVIRONMENT as TellerConnectOptions["environment"];
  const config: TellerConnectOptions = { applicationId: import.meta.env.VITE_TELLER_APPLICATION_ID, environment, products: ["transactions"], selectAccount: "single", nonce, onSuccess };
  const { open, ready } = useTellerConnect(config);
  return <section><button onClick={() => open()} disabled={!ready || !!message}>Connect bank</button>{message && <p>{message}</p>}</section>;
}
