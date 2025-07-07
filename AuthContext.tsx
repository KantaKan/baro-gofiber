import React, { createContext, useState, useContext, useEffect } from "react";
import jwt_decode from "jwt-decode";
import { api } from "./lib/api";

interface AuthContextType {
  isAuthenticated: boolean;
  userRole: string | null;
  error: string | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<string>;
  logout: () => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

interface VerifyTokenResponse {
  status: string;
  data: {
    role: string;
  };
}

export const AuthProvider = ({ children }: { children: React.ReactNode }) => {
  const [isAuthenticated, setIsAuthenticated] = useState(() => !!localStorage.getItem("authToken"));
  const [userRole, setUserRole] = useState<string | null>(() => localStorage.getItem("userRole"));
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const verifyToken = async () => {
      const token = localStorage.getItem("authToken");
      if (token) {
        try {
          const response = await api.get<VerifyTokenResponse>("/api/verify-token", {
            headers: { Authorization: `Bearer ${token}` },
          });

          if (response.data.status === "success") {
            const role = response.data.data.role || "";
            if (!role || role === "invalidRole") {
              throw new Error("Invalid role in response");
            }

            setIsAuthenticated(true);
            setUserRole(role);
            localStorage.setItem("userRole", role);
          } else {
            throw new Error("Token verification failed: Invalid status in response");
          }
        } catch (error) {
          console.error("Token verification failed:", error);
          localStorage.removeItem("authToken");
          localStorage.removeItem("userRole");
          localStorage.removeItem("userId");
          setIsAuthenticated(false);
          setUserRole(null);
        } finally {
          setLoading(false);
        }
      } else {
        setLoading(false);
      }
    };

    verifyToken();
  }, []);

  const login = async (email: string, password: string): Promise<string> => {
    try {
      const response = await api.post("/login", { email, password });

      if (response.data?.data?.token) {
        const role = response.data.data.role || "learner";
        const token = response.data.data.token;

        // Decode the token to get the user_id
        const decoded: any = jwt_decode(token);
        const userId = decoded.user_id;

        // Set token, role, and userId
        localStorage.setItem("authToken", token);
        localStorage.setItem("userRole", role);
        localStorage.setItem("userId", userId);

        // Update auth state
        setIsAuthenticated(true);
        setUserRole(role);
        setError(null);

        // Configure axios with new token
        api.defaults.headers.common["Authorization"] = `Bearer ${token}`;

        return role;
      } else {
        throw new Error("No token in response");
      }
    } catch (error) {
      console.error("Login error:", error);
      setError("Login failed. Please check your credentials.");
      throw error;
    }
  };

  const logout = () => {
    localStorage.removeItem("authToken");
    localStorage.removeItem("userRole");
    localStorage.removeItem("userId");
    setIsAuthenticated(false);
    setUserRole(null);
    setError(null);
  };

  const value: AuthContextType = {
    isAuthenticated,
    userRole,
    error,
    loading,
    login,
    logout,
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return context;
};
