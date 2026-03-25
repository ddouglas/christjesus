exports.onExecutePostLogin = async (event, api) => {
  const namespace = "https://christjesus.app/claims";
  const roles = event.authorization?.roles || [];

  api.idToken.setCustomClaim(`${namespace}/roles`, roles);
  api.accessToken.setCustomClaim(`${namespace}/roles`, roles);

};