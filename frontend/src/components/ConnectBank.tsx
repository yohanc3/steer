import { useCallback } from 'react';

import {
  useTellerConnect,
  type TellerConnectOnSuccess,
  type TellerConnectOnEvent,
  type TellerConnectOnExit,
  type TellerConnectOptions,
} from 'teller-connect-react';

const ConnectBank = () => {
  const applicationId = import.meta.env.TRELLO_APP_ID;
  const onSuccess = useCallback<TellerConnectOnSuccess>((authorization) => {
    // send public_token to your server
    // https://teller.io/docs/api/tokens/#token-exchange-flow
    console.log(authorization);
  }, []);
  const onEvent = useCallback<TellerConnectOnEvent>((name, data) => {
    console.log(name, data);
  }, []);
    asd
  const onExit = useCallback<TellerConnectOnExit>(() => {
    console.log("TellerConnect was dismissed by user");
  }, []);

  const config: TellerConnectOptions = {
    applicationId,
    onSuccess,
    onEvent,
    onExit,
  };

  const {
    open,
    ready,
  } = useTellerConnect(config);


  return (
    <button onClick={() => open()} disabled={!ready}>
      Connect a bank account
    </button>
  );
};

export default ConnectBank;
