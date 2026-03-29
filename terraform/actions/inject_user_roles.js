exports.onExecutePostLogin = async (event, api) => {
  const namespace = "https://christjesus.app/claims";
  const roles = event.authorization?.roles || [];

  api.idToken.setCustomClaim(`${namespace}/roles`, roles);
  api.accessToken.setCustomClaim(`${namespace}/roles`, roles);

  const meta = event.user.user_metadata || {};
  const displayName =
    meta.display_name ||
    meta.given_name ||
    event.user.given_name ||
    event.user.nickname ||
    "";

  api.idToken.setCustomClaim(`${namespace}/display_name`, displayName);
};