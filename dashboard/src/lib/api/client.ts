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
  source_type: 'git' | 'dockerfile' | 'image';
  git_repo?: string;
  git_branch?: string;
  docker_image?: string;
  env_vars?: Record<string, string>;
  port: number;
  status: 'pending' | 'building' | 'deploying' | 'running' | 'stopped' | 'failed';
  container_id?: string;
  subdomain: string;
  custom_domain?: string;
  cpu_limit: number;
  memory_limit: number;
  created_at: string;
  updated_at: string;
}

export interface Database {
  id: string;
  project_id: string;
  name: string;
  engine: 'postgres' | 'redis' | 'mongodb' | 'mysql';
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
  build_logs?: string;
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

    const response = await fetch(url, {
      ...options,
      headers
    });

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
    docker_image?: string;
    port: number;
    env_vars?: Record<string, string>;
  }): Promise<Service> {
    return this.request('/api/services', {
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
    engine: 'postgres' | 'redis' | 'mongodb' | 'mysql';
    version: string;
  }): Promise<Database> {
    return this.request('/api/databases', {
      method: 'POST',
      body: JSON.stringify(data)
    });
  }

  async getDatabaseConnection(id: string): Promise<{
    connection_url: string;
    host: string;
    port: number;
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
}

export const api = new ApiClient();
