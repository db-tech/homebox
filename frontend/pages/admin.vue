<script setup lang="ts">
import type { UserOut } from "~/lib/api/types/data-contracts";
import MdiAccount from "~icons/mdi/account";
import MdiAccountPlus from "~icons/mdi/account-plus";
import MdiShieldAccount from "~icons/mdi/shield-account";
import MdiAccountOff from "~icons/mdi/account-off";
import MdiDelete from "~icons/mdi/delete";
import MdiEmail from "~icons/mdi/email";
import MdiPencil from "~icons/mdi/pencil";

definePageMeta({
  middleware: ["auth"],
});
useHead({
  title: "Homebox | Admin",
});

const auth = useAuthContext();
const api = useUserApi();
const notify = useNotifier();
const confirm = useConfirm();

// Check if user is an admin
const isAdmin = computed(() => auth.user?.isSuperuser);

// If not admin, redirect to home
if (!isAdmin.value) {
  navigateTo("/home");
}

// Get all users
const { data: users, refresh: refreshUsers } = await useAsyncData<UserOut[]>("users", async () => {
  try {
    // Use the real admin API endpoint with explicit credentials option
    console.log("Admin panel: Fetching users from /api/v1/admin/users");
    const response = await fetch('/api/v1/admin/users', {
      credentials: 'include' // Important: include cookies for authentication
    });
    
    console.log("Admin panel: Response status:", response.status);
    
    if (!response.ok) {
      throw new Error(`Error fetching users: ${response.statusText}`);
    }
    
    const data = await response.json();
    console.log("Admin panel: Response data:", data);
    
    // Make sure we're correctly extracting the users array
    const usersList = data.results || [];
    console.log("Admin panel: Extracted users:", usersList.length, "users");
    return usersList;
  } catch (error) {
    console.error("Error fetching users:", error);
    notify.error("Failed to load users");
    return [];
  }
});

// User management
const userDialog = ref(false);
const editingUser = ref<UserOut | null>(null);
const newUser = ref({
  name: "",
  email: "",
  password: "",
  isSuperuser: false,
});

function openUserDialog(user: UserOut | null = null) {
  if (user) {
    editingUser.value = { ...user };
  } else {
    editingUser.value = null;
    newUser.value = {
      name: "",
      email: "",
      password: "",
      isSuperuser: false,
    };
  }
  userDialog.value = true;
}

async function saveUser() {
  if (editingUser.value) {
    // Update existing user
    try {
      // Call the admin API to update the user
      console.log(`Admin panel: Updating user ${editingUser.value.id}`);
      const response = await fetch(`/api/v1/admin/users/${editingUser.value.id}`, {
        method: 'PUT',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          name: editingUser.value.name,
          email: editingUser.value.email,
          isSuperuser: editingUser.value.isSuperuser
        }),
      });
      
      if (!response.ok) {
        throw new Error(`Error updating user: ${response.statusText}`);
      }
      
      notify.success(`User ${editingUser.value.name} updated`);
      userDialog.value = false;
      await refreshUsers();
    } catch (error) {
      console.error("Error updating user:", error);
      notify.error("Failed to update user");
    }
  } else {
    // Create new user
    try {
      // Call the admin API to create the user
      console.log(`Admin panel: Creating new user ${newUser.value.name}`);
      const response = await fetch('/api/v1/admin/users', {
        method: 'POST',
        credentials: 'include',
        headers: {
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({
          name: newUser.value.name,
          email: newUser.value.email,
          password: newUser.value.password,
          isSuperuser: newUser.value.isSuperuser,
          groupID: auth.user?.groupId, // Use current user's group
        }),
      });
      
      if (!response.ok) {
        throw new Error(`Error creating user: ${response.statusText}`);
      }
      
      notify.success(`User ${newUser.value.name} created`);
      userDialog.value = false;
      await refreshUsers();
    } catch (error) {
      console.error("Error creating user:", error);
      notify.error("Failed to create user");
    }
  }
}

async function deleteUser(user: UserOut) {
  if (user.id === auth.user?.id) {
    notify.error("You cannot delete your own account from the admin panel");
    return;
  }

  const result = await confirm.open(
    `Are you sure you want to delete the user ${user.name}? This action cannot be undone.`
  );

  if (result.isCanceled) {
    return;
  }

  try {
    // Call the admin API to delete the user
    console.log(`Admin panel: Deleting user ${user.id}`);
    const response = await fetch(`/api/v1/admin/users/${user.id}`, {
      method: 'DELETE',
      credentials: 'include'
    });
    
    if (!response.ok) {
      throw new Error(`Error deleting user: ${response.statusText}`);
    }
    
    notify.success(`User ${user.name} deleted`);
    await refreshUsers();
  } catch (error) {
    console.error("Error deleting user:", error);
    notify.error("Failed to delete user");
  }
}

const passwordScore = reactive({
  isValid: false,
});
</script>

<template>
  <div>
    <BaseModal v-model="userDialog" size="md">
      <template #title>
        {{ editingUser ? `Edit User: ${editingUser.name}` : "Create New User" }}
      </template>

      <form @submit.prevent="saveUser">
        <div class="space-y-4">
          <template v-if="editingUser">
            <FormTextField 
              v-model="editingUser.name" 
              label="Name" 
              required
            />
            
            <FormTextField 
              v-model="editingUser.email" 
              label="Email" 
              type="email" 
              required
            />
            
            <FormCheckbox 
              v-model="editingUser.isSuperuser" 
              label="Admin User" 
            />
          </template>
          
          <template v-else>
            <FormTextField 
              v-model="newUser.name" 
              label="Name" 
              required
            />
            
            <FormTextField 
              v-model="newUser.email" 
              label="Email" 
              type="email" 
              required
            />
            
            <div class="space-y-2">
              <FormPassword v-model="newUser.password" label="Password" required />
              <PasswordScore v-model:valid="passwordScore.isValid" :password="newUser.password" />
            </div>
            
            <FormCheckbox 
              v-model="newUser.isSuperuser" 
              label="Admin User" 
            />
          </template>

          <div class="flex justify-end space-x-2 pt-4">
            <BaseButton 
              type="button" 
              variant="outline" 
              @click="userDialog = false"
            >
              Cancel
            </BaseButton>
            <BaseButton 
              type="submit" 
              :disabled="!editingUser && !passwordScore.isValid"
            >
              {{ editingUser ? "Update" : "Create" }}
            </BaseButton>
          </div>
        </div>
      </form>
    </BaseModal>

    <BaseContainer>
      <BaseCard>
        <template #title>
          <BaseSectionHeader>
            <MdiShieldAccount class="-mt-1 mr-2" />
            <span>Admin Panel</span>
            <template #description>Manage users and system settings</template>
          </BaseSectionHeader>
        </template>

        <div class="p-4">
          <h2 class="text-xl font-bold">User Management</h2>
          <p class="mb-4 text-sm text-gray-600">Create, edit, and delete users. Only admin users can access this page.</p>

          <div class="mb-4">
            <BaseButton size="sm" class="flex items-center" @click="openUserDialog()">
              <MdiAccountPlus class="mr-1" />
              Add User
            </BaseButton>
          </div>

          <div class="overflow-x-auto">
            <table class="table w-full">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Email</th>
                  <th>Role</th>
                  <th>Actions</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="user in users" :key="user.id">
                  <td class="flex items-center">
                    <MdiAccount class="mr-2" />
                    {{ user.name }}
                  </td>
                  <td>
                    <div class="flex items-center">
                      <MdiEmail class="mr-2" />
                      {{ user.email }}
                    </div>
                  </td>
                  <td>
                    <span v-if="user.isSuperuser" class="badge badge-primary">Admin</span>
                    <span v-else class="badge badge-secondary">User</span>
                  </td>
                  <td>
                    <div class="flex gap-2">
                      <div class="tooltip" data-tip="Edit">
                        <button class="btn btn-square btn-sm" @click="openUserDialog(user)">
                          <MdiPencil />
                        </button>
                      </div>
                      <div class="tooltip" data-tip="Delete">
                        <button 
                          class="btn btn-square btn-sm" 
                          :class="{ 'btn-disabled': user.id === auth.user?.id }"
                          @click="deleteUser(user)"
                        >
                          <MdiDelete />
                        </button>
                      </div>
                    </div>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </BaseCard>
    </BaseContainer>
  </div>
</template>