/**
 * Generated by orval v6.9.6 🍺
 * Do not edit manually.
 * Common Fate
 * Common Fate API
 * OpenAPI spec version: 1.0
 */
import type { IdpStatus } from './idpStatus';

export interface User {
  id: string;
  email: string;
  firstName: string;
  picture: string;
  status: IdpStatus;
  lastName: string;
  updatedAt: string;
  groups: string[];
}
