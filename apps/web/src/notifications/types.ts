export type NotificationChannel = "social" | "campus";

export type NotificationPerson = {
  id: string;
  name: string;
};

export type NotificationDepartment = {
  id: string;
  name: string;
};

export type NotificationItem = {
  id: string;
  resumeId: string;
  candidateName: string;
  department: NotificationDepartment;
  position?: {
    id: string;
    name: string;
  };
  recommender: NotificationPerson;
  chan: NotificationChannel;
  createdAt: string;
  read: boolean;
  canOpenResumeLibrary: boolean;
};

export type NotificationJumpItem = {
  chan: NotificationChannel;
  resumeId: string;
  candidateName: string;
  department: NotificationDepartment;
  recommender: NotificationPerson;
};

export type NotificationJumpContext = {
  items: NotificationJumpItem[];
};
