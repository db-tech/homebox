import { BaseAPI, route } from "../base";
import type { ChangePassword, UserOut } from "../types/data-contracts";
import type { Result } from "../types/non-generated";

export class UserApi extends BaseAPI {
  public self() {
    return this.http.get<Result<UserOut>>({ url: route("/users/self") });
  }

  /**
   * Extends the session in place.
   *
   * A session lasts a week, four with "stay logged in". That is fine for a
   * phone somebody picks up, and useless for a terminal left hanging on a
   * wall: it would log itself out mid-month and the next scan would vanish
   * without an obvious reason. Calling this now and then keeps it alive.
   */
  public refresh() {
    return this.http.get<void>({ url: route("/users/refresh") });
  }

  public logout() {
    return this.http.post<object, void>({ url: route("/users/logout") });
  }

  public delete() {
    return this.http.delete<void>({ url: route("/users/self") });
  }

  public changePassword(current: string, newPassword: string) {
    return this.http.put<ChangePassword, void>({
      url: route("/users/self/change-password"),
      body: {
        current,
        new: newPassword,
      },
    });
  }
}
