import { render, screen } from "@testing-library/react";
import { type FC } from "react";
import { QueryClient, QueryClientProvider } from "react-query";
import { RouterProvider, createMemoryRouter } from "react-router-dom";
import userEvent from "@testing-library/user-event";
import dayjs from "dayjs";
import * as API from "api/api";
import { GlobalSnackbar } from "components/GlobalSnackbar/GlobalSnackbar";
import { ThemeProvider } from "contexts/ThemeProvider";
import { MockTemplate, MockWorkspace } from "testHelpers/entities";
import { WorkspaceScheduleControls } from "./WorkspaceScheduleControls";

const Wrapper: FC = () => {
  return (
    <WorkspaceScheduleControls
      workspace={MockWorkspace}
      template={MockTemplate}
      canUpdateSchedule
    />
  );
};

const BASE_DEADLINE = dayjs().add(3, "hour");

const renderScheduleControls = async () => {
  render(
    <ThemeProvider>
      <QueryClientProvider client={new QueryClient()}>
        <RouterProvider
          router={createMemoryRouter([{ path: "/", element: <Wrapper /> }])}
        />
      </QueryClientProvider>
      <GlobalSnackbar />
    </ThemeProvider>,
  );
  await screen.findByTestId("schedule-controls");
  expect(screen.getByText("Stop in 3 hours")).toBeInTheDocument();
};

test("add 3 hours to deadline", async () => {
  const user = userEvent.setup();
  const updateDeadlineSpy = jest
    .spyOn(API, "putWorkspaceExtension")
    .mockResolvedValue();

  await renderScheduleControls();

  const addButton = screen.getByRole("button", {
    name: /add 1 hour to deadline/i,
  });
  await user.click(addButton);
  await user.click(addButton);
  await user.click(addButton);
  await screen.findByText(
    "Workspace shutdown time has been successfully updated.",
  );
  expect(screen.getByText("Stop in 6 hours")).toBeInTheDocument();

  // Mocks are used here because the 'usedDeadline' is a dayjs object, which
  // can't be directly compared.
  const usedWorkspaceId = updateDeadlineSpy.mock.calls[0][0];
  const usedDeadline = updateDeadlineSpy.mock.calls[0][1];
  expect(usedWorkspaceId).toEqual(MockWorkspace.id);
  expect(usedDeadline.toISOString()).toEqual(
    BASE_DEADLINE.add(3, "hour").toISOString(),
  );
});

test("remove 3 hours to deadline", async () => {
  const user = userEvent.setup();
  const updateDeadlineSpy = jest
    .spyOn(API, "putWorkspaceExtension")
    .mockResolvedValue();

  await renderScheduleControls();

  const subButton = screen.getByRole("button", {
    name: /subtract 1 hour from deadline/i,
  });
  await user.click(subButton);
  await user.click(subButton);
  await screen.findByText(
    "Workspace shutdown time has been successfully updated.",
  );
  expect(screen.getByText("Stop in an hour")).toBeInTheDocument();

  // Mocks are used here because the 'usedDeadline' is a dayjs object, which
  // can't be directly compared.
  const usedWorkspaceId = updateDeadlineSpy.mock.calls[0][0];
  const usedDeadline = updateDeadlineSpy.mock.calls[0][1];
  expect(usedWorkspaceId).toEqual(MockWorkspace.id);
  expect(usedDeadline.toISOString()).toEqual(
    BASE_DEADLINE.subtract(2, "hour").toISOString(),
  );
});

test("rollback to previous deadline on error", async () => {
  const user = userEvent.setup();
  const initialScheduleMessage = "Stop in 3 hours";
  jest.spyOn(API, "putWorkspaceExtension").mockRejectedValue({});

  await renderScheduleControls();

  const addButton = screen.getByRole("button", {
    name: /add 1 hour to deadline/i,
  });
  await user.click(addButton);
  await user.click(addButton);
  await user.click(addButton);
  await screen.findByText(
    "We couldn't update your workspace shutdown time. Please try again.",
  );
  // In case of an error, the schedule message should remain unchanged
  expect(screen.getByText(initialScheduleMessage)).toBeInTheDocument();
});

test("request is only sent once when clicking multiple times", async () => {
  const user = userEvent.setup();
  const updateDeadlineSpy = jest
    .spyOn(API, "putWorkspaceExtension")
    .mockResolvedValue();

  await renderScheduleControls();

  const addButton = screen.getByRole("button", {
    name: /add 1 hour to deadline/i,
  });
  await user.click(addButton);
  await user.click(addButton);
  await user.click(addButton);
  await screen.findByText(
    "Workspace shutdown time has been successfully updated.",
  );
  expect(updateDeadlineSpy).toHaveBeenCalledTimes(1);
});
