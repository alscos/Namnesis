package httpserver

func (s *Server) syncProgram(reason string) {
	if s.state == nil {
		return
	}
	if err := s.state.RefreshProgramNow(reason); err != nil {
		s.state.TriggerProgram(reason + "-retry")
	}
}

func (s *Server) syncPresets(reason string) {
	if s.state == nil {
		return
	}
	if err := s.state.RefreshPresetsNow(reason); err != nil {
		s.state.TriggerPresets(reason + "-retry")
	}
}

func (s *Server) syncProgramAndPresets(reason string) {
	s.syncProgram(reason)
	s.syncPresets(reason)
}
