import {
  AuthenticationDetails,
  CognitoUser,
  CognitoUserPool,
  type CognitoUserSession,
} from "amazon-cognito-identity-js";

import type { AppConfig } from "../config";

// CognitoAuth is the boundary over Amazon Cognito: sign in, read the current token, sign out.
export class CognitoAuth {
  private readonly pool: CognitoUserPool;

  constructor(config: AppConfig) {
    this.pool = new CognitoUserPool({
      UserPoolId: config.cognitoUserPoolId,
      ClientId: config.cognitoClientId,
    });
  }

  // signIn authenticates with SRP and resolves the access token used as the API bearer.
  signIn(email: string, password: string): Promise<string> {
    const user = new CognitoUser({ Username: email, Pool: this.pool });
    const details = new AuthenticationDetails({ Username: email, Password: password });
    return new Promise((resolve, reject) => {
      user.authenticateUser(details, {
        onSuccess: (session) => resolve(session.getAccessToken().getJwtToken()),
        onFailure: (err) => reject(err instanceof Error ? err : new Error(String(err))),
      });
    });
  }

  // currentToken returns a valid access token for the stored session, or null if none.
  currentToken(): Promise<string | null> {
    const user = this.pool.getCurrentUser();
    if (!user) {
      return Promise.resolve(null);
    }
    return new Promise((resolve) => {
      user.getSession((err: Error | null, session: CognitoUserSession | null) => {
        if (err || !session || !session.isValid()) {
          resolve(null);
          return;
        }
        resolve(session.getAccessToken().getJwtToken());
      });
    });
  }

  signOut(): void {
    this.pool.getCurrentUser()?.signOut();
  }
}
