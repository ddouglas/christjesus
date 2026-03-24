exports.onExecutePostLogin = async (event, api) => {
  const namespace = "https://christjesus.app/claims";
  const roles = event.authorization?.roles || [];

  api.idToken.setCustomClaim(`${namespace}/roles`, roles);
  api.accessToken.setCustomClaim(`${namespace}/roles`, roles);

  const displayName =
    event.user.user_metadata?.display_name ||
    event.user.given_name ||
    event.user.nickname ||
    null;
  if (displayName) {
    api.idToken.setCustomClaim(`${namespace}/display_name`, displayName);
  }
};