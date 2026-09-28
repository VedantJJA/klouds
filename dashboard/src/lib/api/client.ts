// Klouds API Client - Deterministic and resilient communication layer

export interface User {
  id: string;
  email: string;
  username: string;
  role: 'admin' | 'user';
  status: 'pending' | 'active' | 'suspended';
  created_at: string;
  updated_at: string;
}

export interface Quota {
  user_id: string;
  max_projects: number;
  max_services: number;
  max_databases: number;
  max_cpu_millicores: number;
  max_memory_mb: number;
  max_disk_mb: number;
}

export interface Project {
  id: string;
  user_id: string;
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
}

export interface Service {
  id: string;
  project_id: string;
  name: string;
  slug?: string;
  type?: string;
  root_dir?: string;
  root_directory?: string;
  build_method?: string;
  source_type?: 'git' | 'dockerfile' | 'image';
  git_repo?: string;
  repo_url?: string;
  git_branch?: string;
  branch?: string;
  docker_image?: string;
  dockerfile_path?: string;
  build_command?: string;
  start_command?: string;
  health_check_path?: string;
  auto_deploy?: boolean;
  env_vars?: Record<string, string>;
  port: number;
  status: 'created' | 'pending' | 'building' | 'deploying' | 'running' | 'stopped' | 'failed';
  container_id?: string;
  image_tag?: string;
  subdomain: string;
  custom_domain?: string;
  runtime_version?: string;
  cpu_limit: number;
  memory_limit: number;
  created_at: string;
  updated_at: string;
}

export interface BlueprintService {
  name: string;
  type: string;
  env: string;
  build_method?: string;
  root_dir?: string;
  build_command?: string;
  start_command?: string;
  dockerfile_path?: string;
  port?: number;
  health_check_path?: string;
  auto_deploy?: boolean;
}

export interface BlueprintDatabase {
  name: string;
  engine: string;
  version?: string;
  database_name?: string;
  user?: string;
}

export interface Blueprint {
  version: string;
  services: BlueprintService[];
  databases?: BlueprintDatabase[];
}

export interface RouteRule {
  id?: string;
  service_id?: string;
  type: 'redirect' | 'rewrite';
  source: string;
  target: string;
  status?: number;
  created_at?: string;
}

export interface OAuthProvider {
  provider: 'github' | 'gitlab' | 'bitbucket';
  name: string;
}

export interface AdminOAuthConfig {
  provider: string;
  client_id: string;
  has_secret: boolean;
  auth_url?: string;
  token_url?: string;
  api_url?: string;
  enabled: boolean;
  updated_at: string;
}

export interface UserOAuthAccount {
  provider: string;
  username: string;
  email: string;
  avatar_url: string;
  created_at: string;
}

export interface GitRepo {
  id: string;
  name: string;
  full_name: string;
  html_url: string;
  clone_url: string;
  default_branch: string;
  private: boolean;
  description: string;
  provider: 'github' | 'gitlab' | 'bitbucket';
  avatar_url?: string;
  updated_at: string;
}

export interface BatchCreateServiceItem {
  name: string;
  type: string;
  build_method: string;
  repo_url?: string;
  branch?: string;
  root_dir?: string;
  dockerfile_path?: string;
  build_command?: string;
  start_command?: string;
  port?: number;
  health_check_path?: string;
  auto_deploy?: boolean;
  runtime_version?: string;
  subdomain?: string;
  env_vars?: Record<string, string>;
  route_rules?: RouteRule[];
}

export interface BatchCreateRequest {
  project_id: string;
  services: BatchCreateServiceItem[];
  deploy?: boolean;
}

export interface BatchCreateResult {
  services: Service[];
  deployments: string[];
}

export interface DetectionResult {
  blueprint: Blueprint;
  source: string;
  detected_branch?: string;
}

export interface Database {
  id: string;
  project_id: string;
  name: string;
  engine: 'postgresql' | 'postgres' | 'redis' | 'mongodb' | 'mysql';
  version: string;
  port: number;
  status: 'creating' | 'running' | 'stopped' | 'failed';
  container_id?: string;
  created_at: string;
  updated_at: string;
}

export interface Deployment {
  id: string;
  service_id: string;
  commit_hash?: string;
  commit_message?: string;
  status: 'queued' | 'building' | 'deploying' | 'active' | 'failed' | 'cancelled';
  build_log?: string;
  build_logs?: string;
  duration_sec?: number;
  created_at: string;
  finished_at?: string;
}

export interface SystemMetrics {
  cpu_percent: number;
  memory_used_mb: number;
  memory_total_mb: number;
  memory_percent: number;
  disk_used_gb: number;
  disk_total_gb: number;
  disk_percent: number;
  uptime_seconds: number;
  load_avg: [number, number, number];
  timestamp: string;
}

const TOKEN_KEY = 'klouds_auth_token';

export function getAuthToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem(TOKEN_KEY);
}

export function setAuthToken(token: string): void {
  if (typeof window === 'undefined') return;
  localStorage.setItem(TOKEN_KEY, token);
}

export function clearAuthToken(): void {
  if (typeof window === 'undefined') return;
  localStorage.removeItem(TOKEN_KEY);
}

class ApiClient {
  private getHeaders(): HeadersInit {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      'Accept': 'application/json'
    };
    const token = getAuthToken();
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
    return headers;
  }

  private async request<T>(path: string, options: RequestInit = {}): Promise<T> {
    const url = path.startsWith('/') ? path : `/${path}`;
    const headers = { ...this.getHeaders(), ...options.headers };

    let response: Response;
    try {
      response = await fetch(url, {
        ...options,
        headers
      });
    } catch (netErr: any) {
      // Retry once for GET requests if transient network disconnection occurs
      if (!options.method || options.method === 'GET') {
        try {
          await new Promise(r => setTimeout(r, 600));
          response = await fetch(url, {
            ...options,
            headers
          });
        } catch {
          throw new Error('Unable to connect to Klouds server. Please verify network connection or server status.');
        }
      } else {
        throw new Error('Unable to connect to Klouds server. Please verify network connection or server status.');
      }
    }

    if (response.status === 401) {
      clearAuthToken();
      if (typeof window !== 'undefined' && !window.location.pathname.startsWith('/login') && !window.location.pathname.startsWith('/register')) {
        window.location.href = '/login';
      }
      throw new Error('Unauthorized');
    }

    if (!response.ok) {
      let errMsg = `Request failed with status ${response.status}`;
      try {
        const errJson = await response.json();
        if (errJson && errJson.error) {
          errMsg = errJson.error;
        }
      } catch {
        // Fall back to default message
      }
      throw new Error(errMsg);
    }

    return response.json();
  }

  // Auth
  async register(data: { email: string; username: string; password: string }): Promise<{ user: User; token: string }> {
    return this.request('/api/auth/register', {
      method: 'POST',
      body: JSON.stringify(data)
    });
  }

  async login(data: { email: string; password: string }): Promise<{ user: User; token: string }> {
    return this.request('/api/auth/login', {
      method: 'POST',
      body: JSON.stringify(data)
    });
  }

  async me(): Promise<{ user: User; quota?: Quota }> {
    return this.request('/api/auth/me');
  }

  // Projects
  async listProjects(): Promise<Project[]> {
    return this.request('/api/projects');
  }

  async getProject(id: string): Promise<Project> {
    return this.request(`/api/projects/${id}`);
  }

  async createProject(data: { name: string; description: string }): Promise<Project> {
    return this.request('/api/projects', {
      method: 'POST',
      body: JSON.stringify(data)
    });
  }

  async deleteProject(id: string): Promise<{ status: string }> {
    return this.request(`/api/projects/${id}`, {
      method: 'DELETE'
    });
  }

  // Services
  async listServices(projectId?: string): Promise<Service[]> {
    const query = projectId ? `?project_id=${encodeURIComponent(projectId)}` : '';
    return this.request(`/api/services${query}`);
  }

  async getService(id: string): Promise<Service> {
    return this.request(`/api/services/${id}`);
  }

  async createService(data: {
    project_id: string;
    name: string;
    source_type: string;
    git_repo?: string;
    git_branch?: string;
    root_dir?: string;
    docker_image?: string;
    port: number;
    env_vars?: Record<string, string>;
  }): Promise<Service> {
    return this.request('/api/services', {
      method: 'POST',
      body: JSON.stringify(data)
    });
  }

  async updateService(id: string, data: {
    name?: string;
    type?: string;
    build_method?: string;
    repo_url?: string;
    branch?: string;
    root_directory?: string;
    dockerfile_path?: string;
    build_command?: string;
    start_command?: string;
    port?: number;
    health_check_path?: string;
    auto_deploy?: boolean;
    env_vars?: Record<string, string>;
  }): Promise<Service> {
    return this.request(`/api/services/${id}`, {
      method: 'PUT',
      body: JSON.stringify(data)
    });
  }

  async getServiceEnv(id: string): Promise<Array<{ id?: string; key: string; value: string; is_build_time?: boolean }>> {
    return this.request(`/api/services/${id}/env`);
  }

  async setServiceEnv(id: string, envVars: Record<string, string>): Promise<{ message: string }> {
    return this.request(`/api/services/${id}/env`, {
      method: 'PUT',
      body: JSON.stringify({ env_vars: envVars })
    });
  }

  async getServiceRoutes(serviceId: string): Promise<RouteRule[]> {
    return this.request(`/api/services/${serviceId}/routes`);
  }

  async setServiceRoutes(serviceId: string, routes: RouteRule[]): Promise<{ message: string; routes: RouteRule[] }> {
    return this.request(`/api/services/${serviceId}/routes`, {
      method: 'PUT',
      body: JSON.stringify({ routes })
    });
  }

  // Blueprints & Monorepo Multi-Service
  async detectBlueprint(projectId: string, repoUrl: string, branch?: string): Promise<DetectionResult> {
    return this.request(`/api/projects/${projectId}/blueprint/detect`, {
      method: 'POST',
      body: JSON.stringify({ repo_url: repoUrl, branch })
    });
  }

  async applyBlueprint(projectId: string, data: { repo_url?: string; branch?: string; yaml_content?: string }): Promise<{
    status: string;
    result: {
      services_created: string[];
      services_updated: string[];
      databases: string[];
      deployments: string[];
    };
  }> {
    return this.request(`/api/projects/${projectId}/blueprint/apply`, {
      method: 'POST',
      body: JSON.stringify(data)
    });
  }

  async stopService(id: string): Promise<{ status: string }> {
    return this.request(`/api/services/${id}/stop`, {
      method: 'POST'
    });
  }

  async restartService(id: string): Promise<{ status: string }> {
    return this.request(`/api/services/${id}/restart`, {
      method: 'POST'
    });
  }

  async deleteService(id: string): Promise<{ status: string }> {
    return this.request(`/api/services/${id}`, {
      method: 'DELETE'
    });
  }

  async getDeployments(serviceId: string): Promise<Deployment[]> {
    return this.request(`/api/services/${serviceId}/deployments`);
  }

  async deployService(serviceId: string): Promise<Deployment> {
    return this.request(`/api/services/${serviceId}/deploy`, {
      method: 'POST'
    });
  }

  // Databases
  async listDatabases(projectId?: string): Promise<Database[]> {
    const query = projectId ? `?project_id=${encodeURIComponent(projectId)}` : '';
    return this.request(`/api/databases${query}`);
  }

  async getDatabase(id: string): Promise<Database> {
    return this.request(`/api/databases/${id}`);
  }

  async createDatabase(data: {
    project_id: string;
    name: string;
    engine: 'postgresql' | 'postgres' | 'redis' | 'mongodb' | 'mysql';
    version: string;
  }): Promise<Database> {
    return this.request('/api/databases', {
      method: 'POST',
      body: JSON.stringify(data)
    });
  }

  async getDatabaseConnection(id: string): Promise<{
    connection_url: string;
    internal_connection_string?: string;
    external_connection_string?: string;
    host: string;
    port: number;
    internal_host?: string;
    external_host?: string;
    external_port?: number;
    username?: string;
    password?: string;
    database?: string;
  }> {
    return this.request(`/api/databases/${id}/connection`);
  }

  async deleteDatabase(id: string): Promise<{ status: string }> {
    return this.request(`/api/databases/${id}`, {
      method: 'DELETE'
    });
  }

  // Admin
  async listUsers(): Promise<User[]> {
    return this.request('/api/admin/users');
  }

  async listPendingUsers(): Promise<User[]> {
    return this.request('/api/admin/users/pending');
  }

  async updateUserStatus(userId: string, status: 'active' | 'suspended'): Promise<User> {
    return this.request(`/api/admin/users/${userId}/status`, {
      method: 'PATCH',
      body: JSON.stringify({ status })
    });
  }

  async deleteUser(userId: string): Promise<{ status: string }> {
    return this.request(`/api/admin/users/${userId}`, {
      method: 'DELETE'
    });
  }

  async getUserQuota(userId: string): Promise<Quota> {
    return this.request(`/api/admin/users/${userId}/quota`);
  }

  async updateUserQuota(userId: string, quota: Partial<Quota>): Promise<Quota> {
    return this.request(`/api/admin/users/${userId}/quota`, {
      method: 'PUT',
      body: JSON.stringify(quota)
    });
  }

  async listAllServices(): Promise<Service[]> {
    return this.request('/api/admin/services');
  }

  async listAllDeployments(): Promise<Deployment[]> {
    return this.request('/api/admin/deployments');
  }

  async getSystemMetrics(): Promise<SystemMetrics> {
    return this.request('/api/admin/metrics/system');
  }

  // OAuth & Git Integration
  async getOAuthProviders(): Promise<OAuthProvider[]> {
    return this.request('/api/auth/oauth/providers');
  }

  async getAdminOAuthConfigs(): Promise<AdminOAuthConfig[]> {
    return this.request('/api/admin/oauth');
  }

  async updateAdminOAuthConfig(provider: string, config: {
    client_id: string;
    client_secret?: string;
    auth_url?: string;
    token_url?: string;
    api_url?: string;
    enabled: boolean;
  }): Promise<{ status: string }> {
    return this.request(`/api/admin/oauth/${provider}`, {
      method: 'PUT',
      body: JSON.stringify(config)
    });
  }

  async getUserOAuthAccounts(): Promise<UserOAuthAccount[]> {
    return this.request('/api/user/oauth');
  }

  async disconnectOAuthAccount(provider: string): Promise<{ status: string }> {
    return this.request(`/api/user/oauth/${provider}`, {
      method: 'DELETE'
    });
  }

  async getGitRepos(): Promise<{ repos: GitRepo[]; connected_providers: string[]; message?: string }> {
    return this.request('/api/git/repos');
  }

  async getGitBranches(repoUrl: string): Promise<string[]> {
    return this.request(`/api/git/branches?repo_url=${encodeURIComponent(repoUrl)}`);
  }

  async createBatchServices(req: BatchCreateRequest): Promise<BatchCreateResult> {
    return this.request('/api/services/batch', {
      method: 'POST',
      body: JSON.stringify(req)
    });
  }
}

export const api = new ApiClient();
