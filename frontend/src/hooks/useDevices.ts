import { useDeviceStore, requestDeviceSelection } from '@/stores/useDeviceStore'
import { refreshDeviceState } from '@/hooks/useDeviceSync'

export function useDevices() {
  const {
    devices,
    activeSerial,
    deviceInfo,
    deviceMode,
    nicknames,
    loading,
    refreshing,
    error,
    lastUpdatedAt,
    setNickname,
  } = useDeviceStore()

  async function refreshDevices() {
    await refreshDeviceState(true)
  }

  // Selection is owned by the shared queue in the device store: several mounted
  // hook instances (top bar, sidebar, devices page) share one serialized worker,
  // so the last user intent always wins instead of the last RPC to return.
  function selectDevice(serial: string): Promise<void> {
    return requestDeviceSelection(serial, 'user')
  }

  return {
    devices,
    activeSerial,
    deviceInfo,
    deviceMode,
    nicknames,
    loading,
    refreshing,
    error,
    lastUpdatedAt,
    setNickname,
    refreshDevices,
    selectDevice,
  }
}
